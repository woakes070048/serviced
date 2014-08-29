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
	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/pool"
	"github.com/control-center/serviced/domain/service"
	"github.com/control-center/serviced/facade"
	"github.com/control-center/serviced/zzk"
)

type remote struct {
	f     *facade.Facade
	realm string
}

func (r *remote) GetResourcePools() ([]*pool.ResourcePool, error) {
	return r.f.GetResourcePoolsByRealm(datastore.Get(), r.realm)
}

func (r *remote) UpdateResourcePool(pool *pool.ResourcePool) error {
	if p, err := r.f.GetResourcePool(datastore.Get(), pool.ID); err != nil {
		return err
	} else if p == nil {
		return r.f.AddResourcePool(datastore.Get(), pool)
	}

	return r.f.UpdateResourcePool(datastore.Get(), pool)
}

func (r *remote) RemoveResourcePool(poolID string) error {
	return r.f.RemoveResourcePool(datastore.Get(), poolID)
}

func (r *remote) GetServices(poolID string) ([]*service.Service, error) {
	return r.f.GetServicesByPool(datastore.Get(), poolID)
}

func (r *remote) UpdateService(service *service.Service) error {
	if s, err := r.f.GetService(datastore.Get(), service.ID); err != nil {
		return err
	} else if s == nil {
		return r.f.AddService(datastore.Get(), *service)
	}

	return r.f.UpdateService(datastore.Get(), *service)
}

func (r *remote) RemoveService(serviceID string) error {
	return r.f.RemoveService(datastore.Get(), serviceID)
}

func (r *remote) StartSync(shutdown <-chan interface{}) {
	for {
		var conn client.Connection
		connc := remoteConnect("/")

		select {
		case conn = <-connc:
			if conn == nil {
				return
			}
		case <-shutdown:
			return
		}

		// Do some synchronization

		select {
		case <-shutdown:
			return
		default:
		}
	}
}

func remoteConnect(path string) <-chan client.Connection {
	connc := make(chan client.Connection)
	go func() {
		for {
			conn, err := zzk.GetRemoteConnection(path)
			if err == zzk.ErrNotInitialized {
				close(connc)
				return
			} else if err == nil {
				connc <- conn
				return
			}
		}
	}()
	return connc
}
