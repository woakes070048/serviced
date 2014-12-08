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

// A modification of docker's own network proxy that will allow us to set the
// packet headers so the source IP won't change.
package proxy

import (
	"fmt"
	"net"

	dockerproxy "github.com/docker/docker/pkg/proxy"
)

func NewProxy(frontendAddr, backendAddr net.Addr) (dockerproxy.Proxy, error) {
	switch frontendAddr.(type) {
	case *net.UDPAddr:
		return NewUDPProxy(frontendAddr.(*net.UDPAddr), backendAddr.(*net.UDPAddr))
	case *net.TCPAddr:
		return NewTCPProxy(frontendAddr.(*net.TCPAddr), backendAddr.(*net.TCPAddr))
	default:
		return nil, fmt.Errorf("Unsupported protocol")
	}
	return nil, nil
}
