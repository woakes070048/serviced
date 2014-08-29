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
package service

import (
	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/domain/service"
	"github.com/control-center/serviced/zzk"
)

type ServiceSyncHandler interface {
	GetServices(poolID string) ([]*service.Service, error)
	UpdateService(service *service.Service) error
	RemoveService(serviceID string) error
}

type ServiceSync struct {
	ServiceSyncHandler
	poolID string
}

func (sync *ServiceSync) SubListeners(nodeID string) []zzk.Listener { return []zzk.Listener{} }

func (sync *ServiceSync) GetPath(nodes ...string) string { return servicepath(nodes...) }

func (sync *ServiceSync) GetAll() ([]zzk.Node, error) {
	svcs, err := sync.GetServices(sync.poolID)
	if err != nil {
		return nil, err
	}

	nodes := make([]zzk.Node, len(svcs))
	for i, svc := range svcs {
		nodes[i] = &ServiceNode{Service: svc}
	}

	return nodes, err
}

func (sync *ServiceSync) Update(node zzk.Node) error {
	return sync.UpdateService(node.(*ServiceNode).Service)
}

func (sync *ServiceSync) Delete(nodeID string) error {
	return sync.RemoveService(nodeID)
}

func NewSyncServicesListener(conn client.Connection, handler ServiceSyncHandler, poolID string) zzk.Listener {
	serviceSync := &ServiceSync{handler, poolID}
	return zzk.NewSyncListener(conn, serviceSync)
}
