// Copyright 2014, The Serviced Authors. All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package zfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"

	"github.com/zenoss/glog"
	"github.com/zenoss/serviced/volume"
)

const (
	DriverName = "zfs"
)

type ZfsDriver struct {
	sudoer bool
}

type ZfsConn struct {
	ZfsDriver
	name string
	root string
}

func init() {
	zfsdriver, err := New()
	if err != nil {
		glog.Error("Can't create zfs driver", err)
		return
	}

	volume.Register(DriverName, zfsdriver)
}

func New() (*ZfsDriver, error) {
	user, err := user.Current()
	if err != nil {
		return nil, err
	}

	result := &ZfsDriver{}
	if user.Uid != "0" {
		err := exec.Command("sudo", "-n", "zfs", "help").Run()
		result.sudoer = err == nil
	}

	return result, nil
}

func (d *ZfsDriver) Mount(volumeName, rootDir string) (volume.Conn, error) {
	if dirp, err := volume.IsDir(rootDir); err != nil || dirp == false {
		if err := os.MkdirAll(rootDir, 0775); err != nil {
			glog.Errorf("Volume root cannot be created: %s", rootDir)
			return nil, err
		}
	}

	v := path.Join(rootDir, volumeName)
	switch exists, err := d.volumeExists(v); {
	case err != nil:
		glog.Errorf("ZFS error: %v", err)
		return nil, fmt.Errorf("ZFS error: %v", err)
	case exists == false:
		// create filesystem for volume
	}

	c := &ZfsConn{d, name: volumeName, root: rootDir}
	return c, nil
}

func (d *ZfsDriver) volumeExists(v string) (bool, error) {
	out, _ := exec.Command("sudo", "zfs", "list").CombinedOutput()
	scanner := bufio.NewScanner(bytes.NewBuffer(out))
	for scanner.Scan() {
		matched, _ := regexp.MatchString(v, scanner.Text())
		if matched {
			return true
		}
	}

	return false
}

func (c *ZfsConn) Name() string {
	return c.name
}

func (c *ZfsConn) Path() string {
	return path.Join(c.root, c.name)
}

func (c *ZfsConn) Snapshot(label string) error {
	return fmt.Errorf("TBI")
}

func (c *ZfsConn) Snapshots() ([]string, error) {
	return []string{}, fmt.Errorf("TBI")
}

func (c *ZfsConn) RemoveSnapshot(lable string) error {
	return fmt.Errorf("TBI")
}

func (c *ZfsConn) Rollback(lable string) error {
	return fmt.Errorf("TBI")
}
