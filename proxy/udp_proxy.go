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
	"encoding/binary"
	"net"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"

	"github.com/zenoss/glog"
	"golang.org/x/net/ipv4"
)

const (
	UDPConnTrackTimeout = 90 * time.Second
	UDPBufSize          = 65507
)

// A net.Addr where the IP is split into two fields so you can use it as a key
// in a map:
type connTrackKey struct {
	IPHigh uint64
	IPLow  uint64
	Port   int
}

func newConnTrackKey(addr *net.UDPAddr) *connTrackKey {
	if len(addr.IP) == net.IPv4len {
		return &connTrackKey{
			IPHigh: 0,
			IPLow:  uint64(binary.BigEndian.Uint32(addr.IP)),
			Port:   addr.Port,
		}
	}
	return &connTrackKey{
		IPHigh: binary.BigEndian.Uint64(addr.IP[:8]),
		IPLow:  binary.BigEndian.Uint64(addr.IP[8:]),
		Port:   addr.Port,
	}
}

type connTrackMap map[connTrackKey]*net.UDPConn

type UDPProxy struct {
	listener       net.PacketConn
	frontendAddr   *net.UDPAddr
	backendAddr    *net.UDPAddr
	connTrackTable connTrackMap
	connTrackLock  sync.Mutex
}

func NewUDPProxy(frontendAddr, backendAddr *net.UDPAddr) (*UDPProxy, error) {
	listener, err := net.ListenIP("ip4:udp", &net.IPAddr{IP: frontendAddr.IP})
	if err != nil {
		return nil, err
	}
	if err := bindToPort(listener, frontendAddr); err != nil {
		listener.Close()
		return nil, err
	}
	return &UDPProxy{
		listener:       listener,
		frontendAddr:   frontendAddr,
		backendAddr:    backendAddr,
		connTrackTable: make(connTrackMap),
	}, nil
}

func (proxy *UDPProxy) replyLoop(proxyConn *net.UDPConn, clientAddr *net.UDPAddr, clientKey *connTrackKey) {
	defer func() {
		proxy.connTrackLock.Lock()
		delete(proxy.connTrackTable, *clientKey)
		proxy.connTrackLock.Unlock()
		proxyConn.Close()
	}()

	readBuf := make([]byte, UDPBufSize)
	for {
		proxyConn.SetReadDeadline(time.Now().Add(UDPConnTrackTimeout))
	again:
		read, err := proxyConn.Read(readBuf)
		if err != nil {
			if err, ok := err.(*net.OpError); ok && err.Err == syscall.ECONNREFUSED {
				// This will happen if the last write failed
				// (e.g: nothing is actually listening on the
				// proxied port on the container), ignore it
				// and continue until UDPConnTrackTimeout
				// expires:
				goto again
			}
			return
		}
		for i := 0; i != read; {
			written, err := proxyConn.WriteToUDP(readBuf[i:read], clientAddr)
			if err != nil {
				return
			}
			i += written
		}
	}
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

func bindToPort(c *net.IPConn, addr *net.UDPAddr) error {
	sockaddr, err := ipToSockaddr(addr.IP, addr.Port)
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

func (proxy *UDPProxy) Run() {
	readBuf := make([]byte, 65535)
	rawConn, _ := ipv4.NewRawConn(proxy.listener)
	for {
		hdr, payload, _, err := rawConn.ReadFrom(readBuf)
		if err != nil {
			if !isClosedError(err) {
				glog.Infof("Stopping proxy on udp/%v for udp/%v (%s)", proxy.frontendAddr, proxy.backendAddr, err)
			}
			break
		}
		packet := gopacket.NewPacket(readBuf, layers.LayerTypeIPv4, gopacket.Default)
		udpLayer := packet.TransportLayer()
		glog.Infof("Received packet layer %+v", udpLayer)
		if udpLayer == nil {
			glog.Infof("Received non-UDP packet")
			continue
		}
		udp, _ := udpLayer.(*layers.UDP)
		from := &net.UDPAddr{IP: hdr.Src, Port: int(udp.SrcPort)}

		fromKey := newConnTrackKey(from)
		proxy.connTrackLock.Lock()
		proxyConn, hit := proxy.connTrackTable[*fromKey]
		if !hit {
			proxyConn, err = net.DialUDP("udp", nil, proxy.backendAddr)
			if err != nil {
				glog.Infof("Can't proxy a datagram to udp/%s: %s\n", proxy.backendAddr, err)
				proxy.connTrackLock.Unlock()
				continue
			}
			proxy.connTrackTable[*fromKey] = proxyConn
			go proxy.replyLoop(proxyConn, from, fromKey)
		}
		proxy.connTrackLock.Unlock()

		// Reroute to the correct IP
		hdr.Dst = proxy.backendAddr.IP
		if err := rawConn.WriteTo(hdr, payload, nil); err != nil {
			//if err := rawConn.WriteTo(hdr, payload, nil); err != nil {
			glog.Infof("Can't proxy a datagram to udp/%s: %s\n", proxy.backendAddr, err)
		}
	}
}

func (proxy *UDPProxy) Close() {
	proxy.listener.Close()
	proxy.connTrackLock.Lock()
	defer proxy.connTrackLock.Unlock()
	for _, conn := range proxy.connTrackTable {
		conn.Close()
	}
}

func (proxy *UDPProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }
func (proxy *UDPProxy) BackendAddr() net.Addr  { return proxy.backendAddr }

func isClosedError(err error) bool {
	/* This comparison is ugly, but unfortunately, net.go doesn't export errClosing.
	 * See:
	 * http://golang.org/src/pkg/net/net.go
	 * https://code.google.com/p/go/issues/detail?id=4337
	 * https://groups.google.com/forum/#!msg/golang-nuts/0_aaCvBmOcM/SptmDyX1XJMJ
	 */
	return strings.HasSuffix(err.Error(), "use of closed network connection")
}
