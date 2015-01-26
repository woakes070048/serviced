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

package scheduler

import (
	"fmt"

	"github.com/control-center/serviced/commons"
	coordclient "github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/dao"
	"github.com/control-center/serviced/domain/addressassignment"
	"github.com/control-center/serviced/domain/host"
	"github.com/control-center/serviced/domain/service"
	"github.com/control-center/serviced/zzk"
	zkservice "github.com/control-center/serviced/zzk/service"
	"github.com/control-center/serviced/zzk/snapshot"
	"github.com/control-center/serviced/zzk/virtualips"
	"github.com/zenoss/glog"
)

type leader struct {
	conn         coordclient.Connection
	dao          dao.ControlPlane
	hostRegistry *zkservice.HostRegistryListener
	poolID       string
}

// Lead is executed by the "leader" of the control center cluster to handle its management responsibilities of:
//    services
//    snapshots
//    virtual IPs
func Lead(shutdown <-chan interface{}, conn coordclient.Connection, dao dao.ControlPlane, poolID string) {
	// creates a listener for the host registry
	if err := zkservice.InitHostRegistry(conn); err != nil {
		glog.Errorf("Could not initialize host registry for pool %s: %s", err)
		return
	}
	hostRegistry := zkservice.NewHostRegistryListener()
	leader := leader{conn, dao, hostRegistry, poolID}
	glog.V(0).Info("Processing leader duties")

	// creates a listener for snapshots with a function call to take snapshots
	// and return the label and error message
	snapshotListener := snapshot.NewSnapshotListener(&leader)

	// creates a listener for services
	serviceListener := zkservice.NewServiceListener(&leader)

	// starts all of the listeners
	zzk.Start(shutdown, conn, serviceListener, hostRegistry, snapshotListener)
}

func (l *leader) TakeSnapshot(serviceID string) (string, error) {
	var label string
	err := l.dao.Snapshot(dao.SnapshotRequest{serviceID, ""}, &label)
	return label, err
}

// SelectHost chooses a host from the pool for the specified service. If the service
// has an address assignment the host will already be selected. If not the host with the least amount
// of memory committed to running containers will be chosen.
func (l *leader) SelectHost(s *service.Service) (*host.Host, error) {
	glog.Infof("Looking for available hosts in pool %s", l.poolID)
	hosts, err := l.hostRegistry.GetHosts()
	if err != nil {
		glog.Errorf("Could not get available hosts for pool %s: %s", l.poolID, err)
		return nil, err
	}

	// make sure all of the endpoints have address assignments
	var assignment *addressassignment.AddressAssignment
	for i, ep := range s.Endpoints {
		if ep.IsConfigurable() {
			if ep.AddressAssignment.IPAddr != "" {
				assignment = &s.Endpoints[i].AddressAssignment
			} else {
				return nil, fmt.Errorf("missing address assignment")
			}
		}
	}
	if assignment != nil {
		glog.Infof("Found an address assignment for %s (%s) at %s, checking host availability", s.Name, s.ID, assignment.IPAddr)
		var hostID string
		var err error

		// Get the hostID from the address assignment
		if assignment.AssignmentType == commons.VIRTUAL {
			if hostID, err = virtualips.GetHostID(l.conn, assignment.IPAddr); err != nil {
				glog.Errorf("Host not available for virtual ip address %s: %s", assignment.IPAddr, err)
				return nil, err
			}
		} else {
			hostID = assignment.HostID
		}

		// Checking host availability
		for i, host := range hosts {
			if host.ID == hostID {
				return hosts[i], nil
			}
		}

		glog.Errorf("Host %s not available in pool %s.  Check to see if the host is running or reassign ips for service %s (%s)", hostID, l.poolID, s.Name, s.ID)
		return nil, fmt.Errorf("host %s not available in pool %s", hostID, l.poolID)
	}

	return NewServiceHostPolicy(s, l.dao).SelectHost(hosts)
}
