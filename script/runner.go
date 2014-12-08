// Copyright 2014, The Serviced Authors. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package script

import (
	"bufio"
	"errors"
	"io"
	"os"

	"github.com/control-center/serviced/commons/docker"
	"github.com/zenoss/glog"
)

var (
	cmdEval map[string]func(*runner, node) error
)

func init() {
	cmdEval = map[string]func(*runner, node) error{
		"":          evalEmpty,
		DESCRIPTION: evalEmpty,
		VERSION:     evalEmpty,
		SNAPSHOT:    evalSnapshot,
		USE:         evalUSE,
		SVC_RUN:     evalSvcRun,
		DEPENDENCY:  evalDependency,
		REQUIRE_SVC: evalRequireSvc,
		SVC_START:   evalSvcStart,
	}
}

type Config struct {
	ServiceID      string
	DockerRegistry string            //docker registry being used for tagging images
	NoOp           bool              //Should commands modify the system
	TenantLookup   TenantIDLookup    //function for looking up a service
	Snapshot       Snapshot          //function for creating snapshots
	Restore        SnapshotRestore   //function to do the rollback to a snapshot
	SvcIDFromPath  ServiceIDFromPath // function to find a service id from a path
	SvcStart       ServiceStart      // function to start a service
}

type Runner interface {
	Run() error
}

type runner struct {
	parseCtx       *parseContext
	config         *Config
	exitFunctions  []func(bool)      //each is called on exit of upgrade, bool denotes if upgrade exited with an error
	snapshotID     string            //the last snapshot taken
	env            map[string]string //context variables available to runner
	tenantIDLookup TenantIDLookup    //function for looking up a service
	snapshot       Snapshot          //function for creating snapshots
	restore        SnapshotRestore   //function to do the rollback to a snapshot
	svcFromPath    ServiceIDFromPath //function to find a service from a path and tenant
	svcStart       ServiceStart      //function to start a service
	findImage      findImage
	pullImage      pullImage
	execCommand    execCmd
	tagImage       tagImage
}

func NewRunnerFromFile(fileName string, config *Config) (Runner, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(f)
	return NewRunner(r, config)
}

func NewRunner(r io.Reader, config *Config) (Runner, error) {
	pctx, err := parseDescriptor(r)
	if err != nil {
		return nil, err
	}
	if len(pctx.errors) > 0 {
		for _, e := range pctx.errors {
			glog.Errorf("%v", e)
		}

		return nil, errors.New("error parsing script")
	}
	return newRunner(config, pctx), nil
}

func newRunner(config *Config, pctx *parseContext) *runner {
	if config.DockerRegistry == "" {
		config.DockerRegistry = "localhost:5000"
	}
	r := &runner{
		parseCtx:       pctx,
		config:         config,
		exitFunctions:  make([]func(bool), 0),
		env:            make(map[string]string),
		tenantIDLookup: config.TenantLookup,
		snapshot:       config.Snapshot,
		restore:        config.Restore,
		svcFromPath:    config.SvcIDFromPath,
		svcStart:       config.SvcStart,
		findImage:      docker.FindImage,
		pullImage:      docker.PullImage,
		execCommand:    defaultExec,
		tagImage:       defaultTagImage,
	}
	if config.NoOp {
		glog.Infof("creatng no op runner")
		r.execCommand = noOpExec
		r.tagImage = noOpTagImage
		r.restore = noOpRestore
		r.snapshot = noOpSnapshot
		r.pullImage = noOpPull
		r.findImage = noOpFindImage
		r.svcStart = noOpServiceStart
	}

	return r
}

func (r *runner) Run() error {
	if err := r.evalNodes(r.parseCtx.nodes); err != nil {
		return err
	}
	return nil
}

func (r *runner) evalNodes(nodes []node) error {
	failed := true
	defer func() {
		for _, ef := range r.exitFunctions {
			ef(failed)
		}
	}()

	for i, n := range nodes {
		if f, found := cmdEval[n.cmd]; found {
			glog.Infof("executing step %d: %s", i, n.line)
			if err := f(r, n); err != nil {
				glog.Errorf("error executing step %d: %s: %s", i, n.cmd, err)
				return err
			}
		} else {
			glog.Infof("skipping step %d unknown function: %s", i, n.line)
		}
	}
	failed = false
	return nil
}
func (r *runner) addExitFunction(ef func(bool)) {
	r.exitFunctions = append(r.exitFunctions, ef)
}