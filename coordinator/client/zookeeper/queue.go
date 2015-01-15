// Copyright 2014 The Serviced Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zookeeper

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/control-center/serviced/coordinator/client"
	zklib "github.com/samuel/go-zookeeper/zk"
)

var ErrEmptyQueue = errors.New("zk: empty queue")

type sortqueue []string

func (q sortqueue) Len() int { return len(q) }
func (q sortqueue) Less(a, b int) bool {
	sa, _ := parseSeq(q[a])
	sb, _ := parseSeq(q[b])
	return sa < sb
}
func (q sortqueue) Swap(a, b int) { q[a], q[b] = q[b], q[a] }

// Queue is the zookeeper construct for FIFO locking queue
type Queue struct {
	c        *Connection // zookeeper connection
	path     string      // path to the queue
	children []string    // children currently enqueued
	lockPath string      // lock that is owned by the queue instance
}

func (q *Queue) prefix() string {
	return join(q.path, "q", "queue-")
}

func (q *Queue) lockPrefix() string {
	return join(q.path, "l", "lock-")
}

func (q *Queue) lock() error {
	var err error
	prefix := q.lockPrefix()
	root := path.Dir(prefix)

	// create a lock node
	lockPath := ""
	for i := 0; i < 3; i++ {
		if q.c.conn == nil {
			return fmt.Errorf("connection lost")
		}

		lockPath, err = q.c.conn.CreateProtectedEphemeralSequential(prefix, []byte{}, zklib.WorldACL(zklib.PermAll))
		if err == zklib.ErrNoNode {
			parts := strings.Split(root, "/")
			pth := ""
			for _, p := range parts[1:] {
				pth += "/" + p
				_, err := q.c.conn.Create(pth, []byte{}, 0, zklib.WorldACL(zklib.PermAll))
				if err != nil && err != zklib.ErrNodeExists {
					return err
				}
			}
		} else if err != nil {
			return err
		} else {
			break
		}
	}

	// get the sequence
	seq, err := parseSeq(lockPath)
	if err != nil {
		return err
	}

	for {
		if q.c.conn == nil {
			// TODO: race condition exists
			return fmt.Errorf("connection lost")
		}

		children, _, err := q.c.conn.Children(root)
		if err != nil {
			return err
		}

		lowestSeq := seq
		prevSeq := uint64(0)
		prevSeqPath := ""
		for _, p := range children {
			s, err := parseSeq(p)
			if err != nil {
				return err
			}
			if s < lowestSeq {
				lowestSeq = s
			}
			if s < seq && s > prevSeq {
				prevSeq = s
				prevSeqPath = p
			}
		}

		if seq == lowestSeq {
			// acquired the lock
			break
		}

		// wait on the node next in line for the lock
		_, _, ch, err := q.c.conn.GetW(join(root, prevSeqPath))
		if err == zklib.ErrNoNode {
			continue
		} else if err != nil {
			return err
		}

		ev := <-ch
		if ev.Err != nil {
			return ev.Err
		}
	}

	q.lockPath = lockPath
	return nil
}

// HasLock returns true when the Queue instance owns the lock
func (q *Queue) HasLock() bool {
	if q.lockPath == "" {
		return false
	}
	ok, _, _ := q.c.conn.Exists(q.lockPath)
	if !ok {
		q.lockPath = ""
	}
	return ok
}

// Put enqueues the desired node and returns the path to the node
func (q *Queue) Put(node client.Node) (string, error) {
	if q.c.conn == nil {
		// TODO: race condition exists
		return "", fmt.Errorf("connection lost")
	}

	data, err := json.Marshal(node)
	if err != nil {
		return "", xlateError(err)
	}
	prefix := q.prefix()
	root := path.Dir(prefix)
	qpath := ""

	// add the node to the queue
	for i := 0; i < 3; i++ {
		if q.c.conn == nil {
			// TODO: race condition exists
			return "", fmt.Errorf("connection lost")
		}

		qpath, err = q.c.conn.Create(prefix, data, zklib.FlagSequence, zklib.WorldACL(zklib.PermAll))
		if err == zklib.ErrNoNode {
			// Create parent node
			parts := strings.Split(root, "/")
			pth := ""
			for _, p := range parts[1:] {
				if q.c.conn == nil {
					// TODO: race condition exists
					return "", fmt.Errorf("connection lost")
				}
				pth += "/" + p
				_, err := q.c.conn.Create(pth, []byte{}, 0, zklib.WorldACL(zklib.PermAll))
				if err != nil && err != zklib.ErrNodeExists {
					return "", xlateError(err)
				}
			}
		} else if err != nil {
			return "", xlateError(err)
		} else {
			break
		}
	}

	return qpath, nil
}

// Get grabs and locks the next node in the queue
func (q *Queue) Get(node client.Node) error {
	if q.c.conn == nil {
		// TODO: race condition exists
		return fmt.Errorf("connection lost")
	}

	if q.HasLock() {
		return ErrDeadlock
	} else if err := q.lock(); err != nil {
		return xlateError(err)
	}

	root := path.Dir(q.prefix())
	for {
		for _, p := range q.children {
			qpath := join(root, p)

			data, stat, err := q.c.conn.Get(qpath)
			if err == zklib.ErrNoNode {
				q.children = q.children[1:]
				continue
			}

			if len(data) > 0 {
				err = json.Unmarshal(data, node)
			} else {
				err = client.ErrEmptyNode
			}

			node.SetVersion(stat)
			return xlateError(err)
		}

		// if no nodes are stored in memory, check zookeeper for the next
		// available node
		for {
			children, _, ch, err := q.c.conn.ChildrenW(root)
			if err != nil {
				return xlateError(err)
			}
			if len(children) > 0 {
				sort.Sort(sortqueue(children))
				q.children = children
				break
			}
			ev := <-ch
			if ev.Err != nil {
				return xlateError(ev.Err)
			}
		}
	}
}

// Consume pops the inflight node off the queue
func (q *Queue) Consume() error {
	if q.c.conn == nil {
		// TODO: race condition exists
		return fmt.Errorf("lost connection")
	}

	if !q.HasLock() {
		return ErrNotLocked
	}

	err := q.c.conn.Delete(path.Dir(q.lockPath), -1)
	return xlateError(err)
}

// Current returns the node at the head of the queue
func (q *Queue) Current(node client.Node) error {
	root := path.Dir(q.prefix())

	for i := 0; i < 3; i++ {
		if q.c.conn == nil {
			// TODO: race condition exists
			return fmt.Errorf("connection lost")
		}

		for _, p := range q.children {
			qpath := join(root, p)
			data, stat, err := q.c.conn.Get(qpath)
			if err == zklib.ErrNoNode {
				q.children = q.children[1:]
				continue
			}
			if len(data) > 0 {
				err = json.Unmarshal(data, node)
			} else {
				err = client.ErrEmptyNode
			}
			node.SetVersion(stat)
			return xlateError(err)
		}
		children, _, err := q.c.conn.Children(q.path)
		if err != nil {
			return xlateError(err)
		}
		if len(children) > 0 {
			sort.Sort(sortqueue(children))
			q.children = children
		} else {
			break
		}
	}
	return ErrEmptyQueue
}
