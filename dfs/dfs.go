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

package dfs

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/facade"
	"github.com/control-center/serviced/zzk"
	zkservice "github.com/control-center/serviced/zzk/service"
	"github.com/zenoss/glog"
)

type DistributedFilesystem struct {
	vfs        string
	dockerHost string
	dockerPort int
	facade     *facade.Facade
	timeout    time.Duration

	// locking
	mutex sync.Mutex
	lock  client.Lock

	// logging
	send func(string)
	recv func() string
	done func()
}

func NewDistributedFilesystem(vfs, dockerRegistry string, facade *facade.Facade, timeout time.Duration) (*DistributedFilesystem, error) {
	host, port, err := parseRegistry(dockerRegistry)
	if err != nil {
		return nil, err
	}

	conn, err := zzk.GetLocalConnection(zzk.GeneratePoolPath("/"))
	if err != nil {
		return nil, err
	}
	lock := zkservice.ServiceLock(conn)

	return &DistributedFilesystem{vfs: vfs, dockerHost: host, dockerPort: port, facade: facade, timeout: timeout, lock: lock}, nil
}

func (dfs *DistributedFilesystem) Lock() error {
	dfs.mutex.Lock()

	err := dfs.lock.Lock()
	if err != nil {
		glog.Warningf("Could not lock services! Operation may be unstable: %s", err)
	}

	dfs.send, dfs.recv, dfs.done = stream()
	return err
}

func (dfs *DistributedFilesystem) Unlock() error {
	defer dfs.mutex.Unlock()
	dfs.done()
	dfs.send, dfs.recv, dfs.done = nil, nil, nil
	return dfs.lock.Unlock()
}

func (dfs *DistributedFilesystem) IsLocked() (bool, error) {
	conn, err := zzk.GetLocalConnection(zzk.GeneratePoolPath("/"))
	if err != nil {
		return false, err
	}
	return zkservice.IsServiceLocked(conn)
}

func (dfs *DistributedFilesystem) GetStatus() string {
	if dfs.recv != nil {
		return dfs.recv()
	}
	return "EOF"
}

func (dfs *DistributedFilesystem) log(msg string, argv ...interface{}) {
	defer glog.V(1).Infof(msg, argv...)
	if dfs.send != nil {
		dfs.send(fmt.Sprintf(msg, argv...))
	}
}

func parseRegistry(registry string) (host string, port int, err error) {
	parts := strings.SplitN(registry, ":", 2)

	if host = parts[0]; host == "" {
		return "", 0, fmt.Errorf("malformed registry")
	} else if len(parts) > 1 {
		if port, err = strconv.Atoi(parts[1]); err != nil {
			return "", 0, fmt.Errorf("malformed registry")
		}
	}
	return host, port, nil
}

func stream() (send func(string), recv func() string, done func()) {
	var messages = make(chan string)
	var q []string
	var mutex sync.Mutex
	var empty sync.WaitGroup

	empty.Add(1)
	go func() {
		for {
			m, ok := <-messages
			if !ok {
				return
			}

			mutex.Lock()
			q = append(q, m)
			if len(q) == 1 {
				empty.Done()
			}
			mutex.Unlock()
		}
	}()

	send = func(m string) { messages <- m }
	recv = func() string {
		empty.Wait()
		mutex.Lock()
		defer mutex.Unlock()
		defer func() {
			q = make([]string, 0)
			empty.Add(1)
		}()
		return q[len(q)-1]
	}
	done = func() { close(messages) }
	return send, recv, done
}
