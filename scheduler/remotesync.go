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

	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/domain/pool"
	localsync "github.com/control-center/serviced/scheduler/sync"
	"github.com/control-center/serviced/sync"
	"github.com/control-center/serviced/zzk"
	"github.com/control-center/serviced/zzk/registry"
	zkpool "github.com/control-center/serviced/zzk/service"
	"github.com/zenoss/glog"
)

func remoteSyncEndpoints(remote, local client.Connection) bool {
	keys, err := new(registry.EndpointSource).KeySource(remote).Get()
	if err != nil {
		glog.Errorf("Could not look up remote endpoints: %s", err)
		return false
	}

	ok := true
	for _, key := range keys {
		endpoints, err := new(registry.EndpointSource).Init(remote, key.GetID()).Get()
		if err != nil {
			glog.Errorf("Could not look up remote endpoints for key %s: %s", key.GetID(), err)
			ok = false
		} else if pass := sync.Synchronize(endpoints, new(registry.EndpointSource).Init(local, key.GetID())); !pass {
			glog.Errorf("Could not synchronize endpoints for key %s", key.GetID())
			ok = false
		}
	}
	return ok
}

func (s *scheduler) remoteSync(shutdown <-chan interface{}, retry time.Duration) {
	for {
		conn, err := zzk.GetLocalConnection("/")
		if err != nil {
			glog.Errorf("Could not establish a local connection to zookeeper: %s", err)
			return
		}

		var masterPool pool.ResourcePool
		event, err := zkpool.WatchResourcePool(conn, s.poolID, &masterPool)
		if err != nil {
			glog.Errorf("Could not monitor master resource pool %s: %s", s.poolID, err)
			return
		}

		ok := func() bool {
			remote, err := zzk.GetRemoteConnection("/")
			if err != nil {
				return false
			}

			// synchronize endpoints
			ok := remoteSyncEndpoints(remote, conn)

			// synchronize resource pools
			pools, err := zkpool.GetResourcePoolsByRealm(remote, masterPool.Realm)
			if err != nil {
				glog.Errorf("Could not get resource pools for realm %s: %s", masterPool.Realm, err)
				ok = false
			}
			for _, pool := range pools {
				remote, err := zzk.GetRemoteConnection(zzk.GeneratePoolPath(pool.ID))
				if err != nil {
					glog.Warningf("Could not acquire a remote pool-based connection to the master %s: %s", pool.ID, err)
					return false
				}

				hosts, err := zkpool.GetHosts(remote)
				if err != nil {
					glog.Errorf("Could not search for hosts in pool %s: %s", pool.ID, err)
					ok = false
				} else if pass := sync.Synchronize(localsync.ConvertHosts(hosts), new(localsync.FacadeHostSource).Init(s.facade, pool.ID)); !pass {
					glog.Errorf("Could not synchronize hosts in pool %s: %s", pool.ID, err)
					ok = false
				}

				svcs, err := zkpool.GetServices(remote)
				if err != nil {
					glog.Errorf("Could not search for services in pool %s: %s", pool.ID, err)
					ok = false
				} else if pass := sync.Synchronize(localsync.ConvertServices(svcs), new(localsync.FacadeServiceSource).Init(s.facade, pool.ID)); !pass {
					glog.Errorf("Could not synchronize services in pool %s: %s", pool.ID, err)
					ok = false
				}
			}

			if pass := sync.Synchronize(localsync.ConvertResourcePools(pools), new(localsync.FacadePoolSource).Init(s.facade, masterPool.Realm)); !pass {
				glog.Errorf("Could not synchronize resource pools for realm %s", masterPool.Realm)
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
		case <-event:
		case <-wait:
		case <-shutdown:
			return
		}
	}
}
