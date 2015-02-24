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

// Package agent implements a service that runs on a serviced node. It is
// responsible for ensuring that a particular node is running the correct services
// and reporting the state and health of those services back to the master
// serviced.

package isvcs

import "github.com/zenoss/glog"

var cephmon *IService

func init() {
	// TODO: missing mon healthcheck
	var err error

	command := "/opt/ceph/bin/ceph-mon.sh"
	cephmon, err = NewIService(
		IServiceDefinition{
			Name:    "ceph-mon",
			Repo:    IMAGE_REPO,
			Tag:     IMAGE_TAG,
			Command: func() string { return command },
			Volumes: map[string]string{
				"lib": "/var/lib/ceph",
			},
			Ports:       []uint16{6789},
			HostNetwork: true,
		})

	if err != nil {
		glog.Fatalf("Error in initializing ceph monitor")
	}
}
