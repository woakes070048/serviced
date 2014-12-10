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
	sa := &syscall.SockaddrInet4{Port: port}
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
	case *net.IPAddr:
		a := addr.(*net.IPAddr)
		return a.IP, 0, nil
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

func RawListener(addr net.Addr) (*net.IPConn, error) {
	switch addr.(type) {
	case *net.UDPAddr:
		// Start listening, then pull the fd out to make an IPConn
		listener, err := net.ListenUDP("udp4", addr.(*net.UDPAddr))
		if err != nil {
			return nil, err
		}
		ptrVal := reflect.ValueOf(listener)
		val := reflect.Indirect(ptrVal)
		// This is a pointer to net.netFD
		fdmember := val.FieldByName("fd")
		conn := net.IPConn{}
		connVal := reflect.ValueOf(conn)
		connVal.FieldByName("fd").Set(fdmember)
		return &conn, nil
	case *net.TCPAddr:
		listener, err := net.ListenTCP("tcp4", addr.(*net.TCPAddr))
		if err != nil {
			return nil, err
		}
		ptrVal := reflect.ValueOf(listener)
		val := reflect.Indirect(ptrVal)
		// This is a pointer to net.netFD
		fdmember := val.FieldByName("fd")
		conn := net.IPConn{}
		connVal := reflect.ValueOf(conn)
		connVal.FieldByName("fd").Set(fdmember)
		return &conn, nil
	}
	return nil, fmt.Errorf("Unknown address type")
}

func NewTransparentProxy(frontendAddr, backendAddr net.Addr) (dockerproxy.Proxy, error) {
	var (
	//frontendIP net.IP
	//frontendPort int
	)
	// This will fail with an err if we don't support the protocol of the frontend address
	//frontendIP, _, err := getAddrInfo(frontendAddr)
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
	//network := frontendAddr.Network() // This will be "tcp" or "udp" at this point
	//listener, err := net.ListenIP("ip4:"+network, &net.IPAddr{IP: frontendIP})
	listener, err := RawListener(frontendAddr)
	if err != nil {
		return nil, err
	}
	// Bind the listener to the frontend port
	//if err := bindToPort(listener, frontendIP, frontendPort); err != nil {
	//	return nil, err
	//}
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
	panic(err)
	glog.Warningf("Unable to start proxy on %s/%v for %s/%v (%s)",
		proxy.frontendAddr.Network(), proxy.frontendAddr,
		proxy.backendAddr.Network(), proxy.backendAddr, err)
}

func (proxy *TransparentProxy) dialBackend() error {
	backendIP, _, err := getAddrInfo(proxy.backendAddr)
	glog.Errorf("LOCAL ADDRESS: %+v", reflect.TypeOf(proxy.localAddr))
	localIP, _, err := getAddrInfo(proxy.localAddr)
	if err != nil {
		glog.Errorf("Shit is so bad -1: %+v", err)
		return err
	}
	network := proxy.backendAddr.Network()
	backendConn, err := net.DialIP("ip4:"+network,
		&net.IPAddr{IP: localIP},
		&net.IPAddr{IP: backendIP})
	if err != nil {
		glog.Errorf("Shit is so bad: %+v", err)
		return err
	}
	rawBackendConn, err := ipv4.NewRawConn(backendConn)
	if err != nil {
		glog.Errorf("Shit is so bad 2: %+v", err)
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

func readPackets(conn *ipv4.RawConn, buffer []byte) (chan packet, error) {
	packetChan := make(chan packet, 1)
	go func() {
		defer close(packetChan)
		for {
			// Read in a packet
			hdr, payload, _, err := conn.ReadFrom(buffer)
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
			if hdr == nil {
				glog.Warningf("Nil header")
				continue
			}
			p := packet{
				Packet:  gopacket.NewPacket(buffer, layers.LayerTypeIPv4, gopacket.Default),
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
			glog.Infof("Received packet destined for %v:%v", p.Dst.IP, p.Dst.Port)
			packetChan <- p
		}
	}()
	return packetChan, nil
}

func (proxy *TransparentProxy) HandleIncoming(exit chan bool) {
	defer proxy.backend.Close()
	buf := make([]byte, MaxIPv4BufferSize)
	packetChan, err := readPackets(proxy.listener, buf)
	if err != nil {
		glog.Infof("Stopping proxy on %s/%v for %s/%v (%s)",
			proxy.frontendAddr.Network(), proxy.frontendAddr,
			proxy.backendAddr.Network(), proxy.backendAddr, err)
		return
	}
	// Rewrite destination to go to the backend
	ip, _, _ := getAddrInfo(proxy.backendAddr)
	for packet := range packetChan {
		// Write it down the pipe
		packet.Header.Dst = ip
		//if packet.Dst.Port != port {
		//	// TODO: Handle this! Involves reserializing the packet
		//	glog.Infof("Proxy port and destination port differ; won't work")
		//}
		proxy.backend.WriteTo(packet.Header, packet.Payload, nil)
	}
	exit <- true
}

func (proxy *TransparentProxy) HandleOutgoing(exit chan bool) {
	buf := make([]byte, MaxIPv4BufferSize)
	packetChan, err := readPackets(proxy.backend, buf)
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
	defer proxy.Close()
	if err := proxy.dialBackend(); err != nil {
		proxy.logError(err)
		return
	}
	exit := make(chan bool)
	go proxy.HandleIncoming(exit)
	go proxy.HandleOutgoing(exit)
	<-exit
}

func (proxy *TransparentProxy) Close() {
	if proxy.listener != nil {
		proxy.listener.Close()
	}
	if proxy.backend != nil {
		proxy.backend.Close()
	}
}

func (proxy *TransparentProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }
func (proxy *TransparentProxy) BackendAddr() net.Addr  { return proxy.backendAddr }
