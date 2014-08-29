// Copyright 2014 The Serviced Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zzk

import (
	"path"
	"sync"
	"time"

	"github.com/control-center/serviced/coordinator/client"
	"github.com/zenoss/glog"
)

// Node manages zookeeper actions
type Node interface {
	client.Node
	// GetID relates to the child node mapping in zookeeper
	GetID() string
	// Create creates the object in zookeeper
	Create(conn client.Connection) error
	// Update updates the object in zookeeper
	Update(conn client.Connection) error
}

// Sync synchronizes zookeeper data with what is in elastic or any other storage facility
func Sync(conn client.Connection, data []Node, zkpath string) error {
	var current []string
	if exists, err := PathExists(conn, zkpath); err != nil {
		return err
	} else if !exists {
		// pass
	} else if current, err = conn.Children(zkpath); err != nil {
		return err
	}

	datamap := make(map[string]Node)
	for i, node := range data {
		datamap[node.GetID()] = data[i]
	}

	for _, id := range current {
		if node, ok := datamap[id]; ok {
			glog.V(2).Infof("Updating id:'%s' at zkpath:%s with: %+v", id, zkpath, node)
			if err := node.Update(conn); err != nil {
				return err
			}
			delete(datamap, id)
		} else {
			glog.V(2).Infof("Deleting id:'%s' at zkpath:%s not found in elastic\nzk current children: %v", id, zkpath, current)
			if err := conn.Delete(path.Join(zkpath, id)); err != nil {
				return err
			}
		}
	}

	for id, node := range datamap {
		glog.V(2).Infof("Creating id:'%s' at zkpath:%s with: %+v", id, zkpath, node)
		if err := node.Create(conn); err != nil {
			return err
		}
	}

	return nil
}

type SyncHandler interface {
	SubListeners(nodeID string) []Listener
	GetPath(nodes ...string) string
	GetAll() ([]Node, error)
	Update(node Node) error
	Delete(string) error
}

type SyncListener struct {
	SyncHandler
	conn client.Connection
}

func NewSyncListener(conn client.Connection, handler SyncHandler) *SyncListener {
	return &SyncListener{handler, conn}
}

func (l *SyncListener) GetConnection() client.Connection { return l.conn }

func (l *SyncListener) Ready() error {
	nodes, err := l.GetAll()
	if err != nil {
		return err
	}

	nodemap := make(map[string]Node)
	for _, node := range nodes {
		nodemap[node.GetID()] = node
	}

	children, err := l.conn.Children(l.GetPath())
	if err != nil {
		return err
	}

	for _, id := range children {
		if node, ok := nodemap[id]; ok {
			delete(nodemap, id)
		} else if err := l.Update(node); err != nil {
			return err
		}
	}

	for id := range nodemap {
		if err := l.Delete(id); err != nil {
			return err
		}
	}
	return nil
}

func (l *SyncListener) Done() { return }

func (l *SyncListener) Spawn(shutdown <-chan interface{}, nodeID string) {
	_shutdown := make(chan interface{})
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()
		Start(_shutdown, nil, l.SubListeners(nodeID)...)
	}()

	defer func() {
		close(_shutdown)
		wg.Wait()
	}()

	for {
		var wait <-chan time.Time

		var node Node
		event, err := l.conn.GetW(l.GetPath(nodeID), node)

		if err == client.ErrNoNode {
			if err := l.Delete(nodeID); err != nil {
				glog.Errorf("Could not delete node at %s: %s", l.GetPath(nodeID), err)
				wait = time.After(time.Minute)
			}
			return
		} else if err != nil {
			glog.Errorf("Could not get node at %s: %s", l.GetPath(nodeID), err)
			return
		} else if err := l.Update(node); err != nil {
			glog.Errorf("Could not update node at %s: %s", l.GetPath(nodeID), err)
			wait = time.After(time.Minute)
		}

		select {
		case <-event:
		case <-wait:
		case <-shutdown:
			return
		}
	}
}
