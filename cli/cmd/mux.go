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

package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/zenoss/glog"
)

// initTemplate is the initializer for serviced mux
func (c *ServicedCli) initMux() {
	c.app.Commands = append(c.app.Commands, cli.Command{
		Name:        "mux",
		Usage:       "runs the serviced mux to proxy connections",
		Description: "",
		Flags: []cli.Flag{
			cli.IntFlag{"p,pid", 0, "pid of the parent process, die when pid goes away"},
		},
		Action: c.mux,
	})
}

// mux runs the serviced mux process
func (c *ServicedCli) mux(ctx *cli.Context) {
	glog.Infof("Got to the mux!")
}
