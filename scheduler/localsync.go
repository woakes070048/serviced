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
package scheduler

import (
	"time"

	"github.com/control-center/serviced/dao"
	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/zzk"

	localsync "github.com/control-center/serviced/scheduler/sync"
	"github.com/control-center/serviced/sync"
	"github.com/zenoss/glog"
)

const minRetry = 15 * time.Second

func (s *scheduler) localSync(shutdown <-chan interface{}, retry time.Duration) {
	for {
		ok := func() bool {
			pools, err := s.facade.GetResourcePools(datastore.Get())
			if err != nil {
				glog.Errorf("Could not get resource pools: %s", err)
				return false
			}

			ok := true
			for _, pool := range pools {
				conn, err := zzk.GetLocalConnection(zzk.GeneratePoolPath(pool.ID))
				if err != nil {
					glog.Errorf("Could not acquire a pool-based connection to zookeeper (%s): %s", pool.ID, err)
					ok = false
					continue
				}

				hosts, err := s.facade.FindHostsInPool(datastore.Get(), pool.ID)
				if err != nil {
					glog.Errorf("Could not search for hosts in pool %s: %s", pool.ID, err)
					ok = false
				} else if pass := sync.Synchronize(localsync.ConvertHosts(hosts), new(localsync.ZKHostSource).Init(conn, 2*time.Minute)); !pass {
					glog.Errorf("Could not synchronize hosts in pool %s", pool.ID)
					ok = false
				}

				svcs, err := s.facade.GetServices(datastore.Get(), dao.ServiceRequest{PoolID: pool.ID})
				if err != nil {
					glog.Errorf("Could not search for services in pool %s: %s", pool.ID, err)
					ok = false
				} else if pass := sync.Synchronize(localsync.ConvertServices(svcs), new(localsync.ZKServiceSource).Init(conn)); !pass {
					glog.Errorf("Could not synchronize services in pool %s", pool.ID)
					ok = false
				}

				if pass := sync.Synchronize(localsync.ConvertVirtualIPs(pool.VirtualIPs), new(localsync.ZKVirtualIPSource).Init(conn)); !pass {
					glog.Errorf("Could not synchronize virtual ips in pool %s", pool.ID)
					ok = false
				}
			}

			conn, err := zzk.GetLocalConnection(zzk.GeneratePoolPath("/"))
			if err != nil {
				glog.Errorf("Could not acquire a connection to zookeeper: %s", err)
				ok = false
			} else if pass := sync.Synchronize(localsync.ConvertResourcePools(pools), new(localsync.ZKPoolSource).Init(conn)); !pass {
				glog.Errorf("Could not synchronize resource pools: %s", err)
				ok = false
			}

			return ok
		}()

		var wait <-chan time.Time
		if !ok {
			glog.Errorf("Could not synchronize, retrying")
			wait = time.After(minRetry)
		} else {
			wait = time.After(retry)
		}

		select {
		case <-wait:
			// pass
		case <-shutdown:
			return
		}
	}
}
