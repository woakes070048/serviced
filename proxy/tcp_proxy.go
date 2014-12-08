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
package proxy

import (
	"io"
	"net"
	"syscall"

	"github.com/zenoss/glog"
)

type TCPProxy struct {
	listener     *net.TCPListener
	frontendAddr *net.TCPAddr
	backendAddr  *net.TCPAddr
}

func NewTCPProxy(frontendAddr, backendAddr *net.TCPAddr) (*TCPProxy, error) {
	listener, err := net.ListenTCP("tcp", frontendAddr)
	if err != nil {
		return nil, err
	}
	// If the port in frontendAddr was 0 then ListenTCP will have a picked
	// a port to listen on, hence the call to Addr to get that actual port:
	return &TCPProxy{
		listener:     listener,
		frontendAddr: listener.Addr().(*net.TCPAddr),
		backendAddr:  backendAddr,
	}, nil
}

func (proxy *TCPProxy) clientLoop(client *net.TCPConn, quit chan bool) {
	backend, err := net.DialTCP("tcp", nil, proxy.backendAddr)
	if err != nil {
		glog.Infof("Can't forward traffic to backend tcp/%v: %s\n", proxy.backendAddr, err)
		client.Close()
		return
	}

	event := make(chan int64)
	var broker = func(to, from *net.TCPConn) {
		written, err := io.Copy(to, from)
		if err != nil {
			// If the socket we are writing to is shutdown with
			// SHUT_WR, forward it to the other end of the pipe:
			if err, ok := err.(*net.OpError); ok && err.Err == syscall.EPIPE {
				from.CloseWrite()
			}
		}
		to.CloseRead()
		event <- written
	}

	go broker(client, backend)
	go broker(backend, client)

	var transferred int64 = 0
	for i := 0; i < 2; i++ {
		select {
		case written := <-event:
			transferred += written
		case <-quit:
			// Interrupt the two brokers and "join" them.
			client.Close()
			backend.Close()
			for ; i < 2; i++ {
				transferred += <-event
			}
			return
		}
	}
	client.Close()
	backend.Close()
}

func (proxy *TCPProxy) Run() {
	quit := make(chan bool)
	defer close(quit)
	for {
		client, err := proxy.listener.Accept()
		if err != nil {
			glog.Infof("Stopping proxy on tcp/%v for tcp/%v (%s)", proxy.frontendAddr, proxy.backendAddr, err)
			return
		}
		go proxy.clientLoop(client.(*net.TCPConn), quit)
	}
}

func (proxy *TCPProxy) Close()                 { proxy.listener.Close() }
func (proxy *TCPProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }
func (proxy *TCPProxy) BackendAddr() net.Addr  { return proxy.backendAddr }
