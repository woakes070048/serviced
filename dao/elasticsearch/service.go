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

package elasticsearch

import (
	"fmt"
	"time"

	"github.com/control-center/serviced/dao"
	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/service"
	"github.com/control-center/serviced/domain/servicestate"
	"github.com/control-center/serviced/zzk"
	zkservice "github.com/control-center/serviced/zzk/service"
	"github.com/zenoss/glog"
)

// AddService add a service. Return error if service already exists
func (this *ControlPlaneDao) AddService(svc service.Service, serviceId *string) error {
	if err := this.facade.AddService(datastore.Get(), svc); err != nil {
		return err
	}
	*serviceId = svc.ID
	return nil
}

//
func (this *ControlPlaneDao) UpdateService(svc service.Service, unused *int) error {
	if err := this.facade.UpdateService(datastore.Get(), svc); err != nil {
		return err
	}
	return nil
}

//
func (this *ControlPlaneDao) RemoveService(id string, unused *int) error {
	if err := this.facade.RemoveService(datastore.Get(), id); err != nil {
		return err
	}
	return nil
}

// GetService gets a service.
func (this *ControlPlaneDao) GetService(id string, myService *service.Service) error {
	svc, err := this.facade.GetService(datastore.Get(), id)
	if svc != nil {
		*myService = *svc
	}
	return err
}

// Get the services (can filter by name and/or tenantID)
func (this *ControlPlaneDao) GetServices(request dao.ServiceRequest, services *[]service.Service) error {
	if svcs, err := this.facade.GetServices(datastore.Get(), request); err == nil {
		*services = svcs
		return nil
	} else {
		return err
	}
}

//
func (this *ControlPlaneDao) FindChildService(request dao.FindChildRequest, service *service.Service) error {
	svc, err := this.facade.FindChildService(datastore.Get(), request.ServiceID, request.ChildName)
	if err != nil {
		return err
	}

	if svc != nil {
		*service = *svc
	} else {
		glog.Warningf("unable to find child of service: %+v", service)
	}
	return nil
}

// Get tagged services (can also filter by name and/or tenantID)
func (this *ControlPlaneDao) GetTaggedServices(request dao.ServiceRequest, services *[]service.Service) error {
	if svcs, err := this.facade.GetTaggedServices(datastore.Get(), request); err == nil {
		*services = svcs
		return nil
	} else {
		return err
	}
}

// The tenant id is the root service uuid. Walk the service tree to root to find the tenant id.
func (this *ControlPlaneDao) GetTenantId(serviceID string, tenantId *string) error {
	if tid, err := this.facade.GetTenantID(datastore.Get(), serviceID); err == nil {
		*tenantId = tid
		return nil
	} else {
		return err
	}
}

// Get a service endpoint.
func (this *ControlPlaneDao) GetServiceEndpoints(serviceID string, response *map[string][]dao.ApplicationEndpoint) (err error) {
	if result, err := this.facade.GetServiceEndpoints(datastore.Get(), serviceID); err == nil {
		*response = result
		return nil
	} else {
		return err
	}
}

// start the provided service
func (this *ControlPlaneDao) StartService(serviceID string, unused *string) error {
	return this.facade.StartService(datastore.Get(), serviceID)
}

// restart the provided service
func (this *ControlPlaneDao) RestartService(serviceID string, unused *int) error {
	return this.facade.RestartService(datastore.Get(), serviceID)
}

// synchronous pause across all running services
func (this *ControlPlaneDao) PauseServices(timeout time.Duration, unused *int) error {
	type status struct {
		id  string
		err error
	}

	cancel := make(chan interface{})
	processing := make(map[string]struct{})
	done := make(chan status)

	var svcs []service.Service
	if err := this.GetServices(dao.ServiceRequest{}, &svcs); err != nil {
		glog.Errorf("Could not get all services: %s", err)
		return err
	}

	for _, svc := range svcs {
		// pause only running service instances
		if svc.DesiredState == service.SVCRun {
			processing[svc.ID] = struct{}{}

			go func(id string) {
				if err := this.facade.PauseService(datastore.Get(), id); err != nil {
					done <- status{id, err}
					return
				}

				conn, err := zzk.GetLocalConnection(zzk.GeneratePoolPath(svc.PoolID))
				if err != nil {
					done <- status{id, err}
					return
				}

				var states []servicestate.ServiceState
				if err := this.GetServiceStates(svc.ID, &states); err != nil {
					done <- status{id, err}
					return
				}

				for _, state := range states {
					if err := zkservice.WaitPause(cancel, conn, id, state.ID); err != nil {
						err = fmt.Errorf("could not pause instance %s (%s): %s", state.ID, id, err)
						done <- status{id, err}
						return

					}
					select {
					case <-cancel:
						err = fmt.Errorf("action cancelled")
						done <- status{id, err}
						return
					default:
					}
				}
			}(svc.ID)
		}
	}

	timeoutC := time.After(timeout)
	defer func() {
		for len(processing) > 0 {
			delete(processing, (<-done).id)
		}
	}()
	for len(processing) > 0 {
		select {
		case status := <-done:
			delete(processing, (<-done).id)
			if status.err != nil {
				close(cancel)
				return status.err
			}
		case <-timeoutC:
			close(cancel)
			return fmt.Errorf("timeout")
		}
	}
	return nil
}

// resumes all paused services
func (this *ControlPlaneDao) ResumeServices(request dao.EntityRequest, unused *int) error {
	var svcs []service.Service
	if err := this.GetServices(dao.ServiceRequest{}, &svcs); err != nil {
		glog.Errorf("Could not get all services: %s", err)
		return err
	}

	for _, svc := range svcs {
		if svc.DesiredState == service.SVCPause {
			if err := this.StartService(svc.ID, nil); err != nil {
				glog.Warningf("Could not resume service %s (%s): %s", svc.Name, svc.ID, err)
				continue
			}
		}
	}

	return nil
}

// stop the provided service
func (this *ControlPlaneDao) StopService(id string, unused *int) error {
	return this.facade.StopService(datastore.Get(), id)
}

// assign an IP address to a service (and all its child services) containing non default AddressResourceConfig
func (this *ControlPlaneDao) AssignIPs(assignmentRequest dao.AssignmentRequest, _ *struct{}) error {
	return this.facade.AssignIPs(datastore.Get(), assignmentRequest)
}
