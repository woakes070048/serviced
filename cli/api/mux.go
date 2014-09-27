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

package api

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/control-center/serviced/commons/subprocess"
	"github.com/control-center/serviced/proxy"
	"github.com/zenoss/glog"
)

type muxController interface {
	Close()
}

type muxInprocess struct {
	mux *proxy.TCPMux
}

func (m *muxInprocess) Close() {
	m.mux.Close()
}

type muxDaemon struct {
	env     []string
	closing chan chan error
}

func (m *muxDaemon) Close() {
	errc := make(chan error)
	m.closing <- errc
	err := <-errc
	if err != nil {
		glog.Errorf("error closing subprocess mux: %s", err)
	}
}

func newMuxController(subprocess bool) (muxController, error) {

	if !subprocess {
		muxListener, err := createMuxListener()
		if err != nil {
			return nil, err
		}
		mux, err := proxy.NewTCPMux(muxListener)
		if err != nil {
			return nil, err
		}
		daemon := &muxInprocess{
			mux: mux,
		}
		return daemon, nil
	}
	return newMuxDaemon()
}

func (m *muxDaemon) muxDaemonCmd() (*subprocess.Instance, chan error, error) {
	return subprocess.New(time.Second*10, m.env, os.Args[0], "mux", "--pid", fmt.Sprintf("%s", os.Getpid()))
}

func newMuxDaemon() (*muxDaemon, error) {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 1)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	if options.TLS {
		env["SERVICED_TLS"] = "1"
	} else {
		env["SERVICED_TLS"] = "0"
	}
	env["SERVICED_MUX"] = fmt.Sprintf("%s", options.MuxPort)
	if len(options.KeyPEMFile) > 0 {
		env["SERVICED_KEYPEMFILE"] = options.KeyPEMFile
	}
	if len(options.CertPEMFile) > 0 {
		env["SERVICED_CERTPEMFILE"] = options.CertPEMFile
	}
	envlist := make([]string, 0)
	for k, v := range env {
		envlist = append(envlist, fmt.Sprintf("%s=%s", k, v))
	}
	daemon := muxDaemon{
		env:     envlist,
		closing: make(chan chan error),
	}
	cmd, exited, err := daemon.muxDaemonCmd()
	if err != nil {
		return nil, err
	}
	go daemon.loop(cmd, exited)
	return &daemon, nil
}

func (a *api) Mux(port int, tls bool, keyfile, certfile string) {

}

func (m *muxDaemon) loop(cmd *subprocess.Instance, exited chan error) {

	var err error
	var restart <-chan time.Time
	for {
		select {
		case err = <-exited:
			glog.Infof("mux daemon exited. restarting...")
			cmd = nil
			restart = time.After(time.Duration(0))
		case <-restart:
			cmd, exited, err = m.muxDaemonCmd()
			if err != nil {
				glog.Errorf("Could not restart mux daemon: %s", err)
				exited = nil
				cmd = nil
				restart = time.After(time.Second * 10)
			}
		case <-m.closing:
			if cmd != nil {
				cmd.Close()
			}
			return
		}
	}
}

func createMuxListener() (net.Listener, error) {
	if options.TLS {
		glog.V(1).Info("using TLS on mux")

		proxyCertPEM, proxyKeyPEM, err := getKeyPairs(options.CertPEMFile, options.KeyPEMFile)
		if err != nil {
			return nil, err
		}

		cert, err := tls.X509KeyPair([]byte(proxyCertPEM), []byte(proxyKeyPEM))
		if err != nil {
			glog.Error("ListenAndMux Error (tls.X509KeyPair): ", err)
			return nil, err
		}

		tlsConfig := tls.Config{Certificates: []tls.Certificate{cert}}
		glog.V(1).Infof("TLS enabled tcp mux listening on %d", options.MuxPort)
		return tls.Listen("tcp", fmt.Sprintf(":%d", options.MuxPort), &tlsConfig)

	}
	return net.Listen("tcp", fmt.Sprintf(":%d", options.MuxPort))
}
