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
	"reflect"
	"syscall"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"

	dockerproxy "github.com/docker/docker/pkg/proxy"
	"github.com/zenoss/glog"
	"golang.org/x/net/ipv4"
)

const (
	MaxIPv4BufferSize = 65535
)

func NewProxy(frontendAddr, backendAddr net.Addr) (dockerproxy.Proxy, error) {
	return NewTransparentProxy(frontendAddr, backendAddr)
}

func ipToSockaddr(ip net.IP, port int) (syscall.Sockaddr, error) {
	if len(ip) == 0 {
		ip = net.IPv4zero
	}
	if ip = ip.To4(); ip == nil {
		return nil, net.InvalidAddrError("non-IPv4 address")
	}
	sa := new(syscall.SockaddrInet4)
	for i := 0; i < net.IPv4len; i++ {
		sa.Addr[i] = ip[i]
	}
	sa.Port = port
	return sa, nil
}

func bindToPort(c *net.IPConn, ip net.IP, port int) error {
	sockaddr, err := ipToSockaddr(ip, port)
	if err != nil {
		return err
	}
	ptrVal := reflect.ValueOf(c)
	val := reflect.Indirect(ptrVal)
	//next line will get you the net.netFD
	fdmember := val.FieldByName("fd")
	val1 := reflect.Indirect(fdmember)
	netFdPtr := val1.FieldByName("sysfd")
	fd := int(netFdPtr.Int())
	return syscall.Bind(fd, sockaddr)
}

func getAddrInfo(addr net.Addr) (ip net.IP, port int, err error) {
	switch addr.(type) {
	case *net.UDPAddr:
		a := addr.(*net.UDPAddr)
		return a.IP, a.Port, nil
	case *net.TCPAddr:
		a := addr.(*net.TCPAddr)
		return a.IP, a.Port, nil
	default:
		return nil, 0, fmt.Errorf("Unsupported protocol")
	}
}

type TransparentProxy struct {
	listener     *ipv4.RawConn
	backend      *ipv4.RawConn
	frontendAddr net.Addr
	backendAddr  net.Addr
	localAddr    net.Addr
}

func NewTransparentProxy(frontendAddr, backendAddr net.Addr) (dockerproxy.Proxy, error) {
	var (
		frontendIP   net.IP
		frontendPort int
	)
	// This will fail with an err if we don't support the protocol of the frontend address
	frontendIP, frontendPort, err := getAddrInfo(frontendAddr)
	// Verify that both addresses use the same protocol
	switch frontendAddr.(type) {
	case *net.UDPAddr:
		if _, ok := backendAddr.(*net.UDPAddr); !ok {
			return nil, fmt.Errorf("Frontend and backend addresses have different protocols")
		}
	case *net.TCPAddr:
		if _, ok := backendAddr.(*net.TCPAddr); !ok {
			return nil, fmt.Errorf("Frontend and backend addresses have different protocols")
		}
	}
	// Open a raw socket for incoming traffic
	network := frontendAddr.Network() // This will be "tcp" or "udp" at this point
	listener, err := net.ListenIP("ip4:"+network, &net.IPAddr{IP: frontendIP})
	if err != nil {
		return nil, err
	}
	// Bind the listener to the frontend port
	if err := bindToPort(listener, frontendIP, frontendPort); err != nil {
		return nil, err
	}
	rawClientConn, err := ipv4.NewRawConn(listener)
	if err != nil {
		return nil, err
	}
	return &TransparentProxy{
		listener:     rawClientConn,
		frontendAddr: frontendAddr,
		backendAddr:  backendAddr,
		localAddr:    listener.LocalAddr(),
	}, nil
}

func (proxy *TransparentProxy) logError(err error) {
	glog.Warningf("Unable to start proxy on %s/%v for %s/%v (%s)",
		proxy.frontendAddr.Network(), proxy.frontendAddr,
		proxy.backendAddr.Network(), proxy.backendAddr, err)
}

var la = &net.IPAddr{
	IP: net.IP{127, 0, 0, 1},
}

func (proxy *TransparentProxy) dialBackend() error {
	backendIP, backendPort, err := getAddrInfo(proxy.backendAddr)
	if err != nil {
		return err
	}
	network := proxy.backendAddr.Network()
	backendConn, err := net.DialIP("ip4:"+network, la, &net.IPAddr{IP: backendIP})
	if err != nil {
		return err
	}
	rawBackendConn, err := ipv4.NewRawConn(backendConn)
	if err != nil {
		return err
	}
	proxy.backend = rawBackendConn
	return nil
}

type address struct {
	IP   net.IP
	Port int
}

type packet struct {
	Packet  gopacket.Packet
	Header  *ipv4.Header
	Payload []byte
	Src     *address
	Dst     *address
}

func readPackets(conn *ipv4.RawConn, buffer *[]byte) (chan packet, error) {
	packetChan := make(chan packet, 1)
	go func() {
		defer close(packetChan)
		for {
			// Read in a packet
			hdr, payload, _, err := conn.ReadFrom(*buffer)
			if err != nil {
				switch v := err.(type) {
				case *net.OpError:
					if v.Timeout() {
						continue
					}
				case *net.AddrError:
					if v.Timeout() {
						continue
					}
				case *net.UnknownNetworkError:
					if v.Timeout() {
						continue
					}
				default:
					glog.Errorf("Proxy connection error: %+v", err)
					return
				}
			}
			p := packet{
				Packet:  gopacket.NewPacket(*buffer, layers.LayerTypeIPv4, gopacket.Default),
				Header:  hdr,
				Payload: payload,
				Src:     &address{IP: hdr.Src},
				Dst:     &address{IP: hdr.Dst},
			}
			transport := p.Packet.TransportLayer()
			switch transport.(type) {
			case *layers.UDP:
				udpLayer := transport.(*layers.UDP)
				p.Src.Port = int(udpLayer.SrcPort)
				p.Dst.Port = int(udpLayer.DstPort)
			case *layers.TCP:
				tcpLayer := transport.(*layers.TCP)
				p.Src.Port = int(tcpLayer.SrcPort)
				p.Dst.Port = int(tcpLayer.DstPort)
			default:
				glog.V(4).Infof("Received packet with unknown protocol %s. Ignoring.",
					transport.LayerType())
				continue
			}
			packetChan <- p
		}
	}()
	return packetChan, nil
}

func (proxy *TransparentProxy) HandleIncoming(exit chan bool) {
	defer proxy.backend.Close()
	buf := make([]byte, MaxIPv4BufferSize)
	packetChan, err := readPackets(proxy.listener, &buf)
	if err != nil {
		glog.Infof("Stopping proxy on %s/%v for %s/%v (%s)",
			proxy.frontendAddr.Network(), proxy.frontendAddr,
			proxy.backendAddr.Network(), proxy.backendAddr, err)
		return
	}
	// Rewrite destination to go to the backend
	ip, port, _ := getAddrInfo(proxy.backendAddr)
	for packet := range packetChan {
		// Write it down the pipe
		packet.Header.Dst = ip
		if packet.Dst.Port != port {
			// TODO: Handle this! Involves reserializing the packet
			glog.Infof("Proxy port and destination port differ; won't work")
		}
		proxy.backend.WriteTo(packet.Header, packet.Payload, nil)
	}
	exit <- true
}

func (proxy *TransparentProxy) HandleOutgoing(exit chan bool) {
	buf := make([]byte, MaxIPv4BufferSize)
	packetChan, err := readPackets(proxy.backend, &buf)
	if err != nil {
		glog.Infof("Stopping proxy on %s/%v for %s/%v (%s)",
			proxy.frontendAddr.Network(), proxy.frontendAddr,
			proxy.backendAddr.Network(), proxy.backendAddr, err)
		return
	}
	// Rewrite source to be the proxy
	ip, _, _ := getAddrInfo(proxy.localAddr)
	for packet := range packetChan {
		// Write it down the pipe
		packet.Header.Src = ip
		proxy.listener.WriteTo(packet.Header, packet.Payload, nil)
	}
	exit <- true
}

func (proxy *TransparentProxy) Run() {
	proxy.dialBackend()

	exit := make(chan bool)

	go proxy.HandleIncoming(exit)
	go proxy.HandleOutgoing(exit)

	<-exit
	proxy.Close()

}

func (proxy *TransparentProxy) Close() {
	proxy.listener.Close()
	proxy.backend.Close()
}

func (proxy *TransparentProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }
func (proxy *TransparentProxy) BackendAddr() net.Addr  { return proxy.backendAddr }
