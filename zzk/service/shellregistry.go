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
)

const zkShell = "/shell"

func shellregpath(nodes ...string) string {
	p := append([]string{zkRegistry, zkShell}, nodes...)
	return path.Clean(path.Join(p...))
}

type ShellNode struct {
	HostIP      string
	ServiceID   string
	ContainerID string
	version     interface{}
}

func (node *ShellNode) Version() interface{} {
	return node.version
}

func (node *ShellNode) SetVersion(version interface{}) {
	node.version = version
}

func RegisterShell(conn client.Connection, serviceID, containerID, hostIP string) error {
	node := ShellNode{HostIP: hostIP, ContainerID: containerID, ServiceID: serviceID}
	_, err := conn.CreateEphemeral(shellregpath(serviceID, containerID), &node)
	return err
}

func FindShells(conn client.Connection, serviceID string) ([]ShellNode, error) {
	eShells, err := conn.Children(shellregpath(serviceID))

	var nodes []ShellNode

	if err == client.ErrNoNode {
		return nodes, nil
	} else if err != nil {
		return nil, err
	}

	for _, id := range eShells {
		var node ShellNode
		if err := conn.Get(shellregpath(serviceID, id), &node); err == client.ErrNoNode {
			continue
		} else if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}
