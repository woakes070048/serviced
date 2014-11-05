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
package service

import (
	"path"

	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/domain/pool"
	"github.com/control-center/serviced/zzk"
	"github.com/zenoss/glog"
)

const (
	zkPool = "/pools"
)

func poolpath(nodes ...string) string {
	p := append([]string{zkPool}, nodes...)
	return path.Join(p...)
}

type PoolNode struct {
	*pool.ResourcePool
	version interface{}
}

// GetID implements sync.Datum
func (node PoolNode) GetID() string {
	return node.ID
}

// Version implements client.Node
func (node *PoolNode) Version() interface{} { return node.version }

// SetVersion implements client.Node
func (node *PoolNode) SetVersion(version interface{}) { node.version = version }

func GetResourcePools(conn client.Connection) ([]pool.ResourcePool, error) {
	nodes, err := conn.Children(poolpath())
	if err != nil {
		return nil, err
	}

	pools := make([]pool.ResourcePool, len(nodes))
	for i, poolID := range nodes {
		var node PoolNode
		if err := conn.Get(poolpath(poolID), &node); err != nil {
			return nil, err
		}
		pools[i] = *node.ResourcePool
	}

	return pools, nil
}

func GetResourcePoolsByRealm(conn client.Connection, realm string) ([]pool.ResourcePool, error) {
	allpools, err := GetResourcePools(conn)
	if err != nil {
		return nil, err
	}

	var pools []pool.ResourcePool
	for _, pool := range allpools {
		if pool.Realm == realm {
			pools = append(pools, pool)
		}
	}

	return pools, nil
}

func AddResourcePool(conn client.Connection, pool *pool.ResourcePool) error {
	var node PoolNode
	if err := conn.Create(poolpath(pool.ID), &node); err != nil {
		return err
	}
	node.ResourcePool = pool
	return conn.Set(poolpath(pool.ID), &node)
}

func UpdateResourcePool(conn client.Connection, pool *pool.ResourcePool) error {
	var node PoolNode
	if err := conn.Get(poolpath(pool.ID), &node); err != nil {
		return err
	}
	node.ResourcePool = pool
	return conn.Set(poolpath(pool.ID), &node)
}

func RemoveResourcePool(conn client.Connection, poolID string) error {
	return conn.Delete(poolpath(poolID))
}

func WatchResourcePool(conn client.Connection, poolID string, pool *pool.ResourcePool) (<-chan client.Event, error) {
	return conn.GetW(poolpath(poolID), &PoolNode{ResourcePool: pool})
}

func MonitorResourcePool(shutdown <-chan interface{}, conn client.Connection, poolID string) <-chan *pool.ResourcePool {
	monitor := make(chan *pool.ResourcePool)
	go func() {
		defer close(monitor)
		if err := zzk.Ready(shutdown, conn, poolpath(poolID)); err != nil {
			glog.V(2).Infof("Could not watch pool %s: %s", poolID, err)
			return
		}
		for {
			var node PoolNode
			event, err := conn.GetW(poolpath(poolID), &node)
			if err != nil {
				glog.V(2).Infof("Could not get pool %s: %s", poolID, err)
				return
			}

			select {
			case monitor <- node.ResourcePool:
			case <-shutdown:
				return
			}

			select {
			case <-event:
			case <-shutdown:
				return
			}
		}
	}()
	return monitor
}
