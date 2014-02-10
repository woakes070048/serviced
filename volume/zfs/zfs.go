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
	pool string
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
		if _, err = runcmd(d.sudoer, "create", "-o", fmt.Sprintf("mountpoint=%s", v), fmt.Sprintf("zenpool/%s", volumeName)); err != nil {
			glog.Errorf("Could not create filesystem for: %s", v)
			return nil, fmt.Errorf("Could no create filesystem for: %s (%v)", v, err)
		}
	}

	c := &ZfsConn{*d, fmt.Sprintf("zenpool/%s", volumeName), volumeName, rootDir}
	return c, nil
}

func (c *ZfsConn) Name() string {
	return c.name
}

func (c *ZfsConn) Path() string {
	return path.Join(c.root, c.name)
}

func (c *ZfsConn) Snapshot(label string) error {
	_, err := runcmd(c.sudoer, "zfs", "snapshot", fmt.Sprintf("%s@%s", c.pool, label))
	return err
}

func (c *ZfsConn) Snapshots() ([]string, error) {
	cmdout, err := exec.Command("sudo", "ls", path.Join(c.Path(), ".zfs", "snapshot")).CombinedOutput()
	if err != nil {
		errmsg := fmt.Errorf("Can't open snapshots directory: %v", err)
		glog.Errorf("%v", errmsg)
		return []string{}, errmsg
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(cmdout))
	snaps := []string{}
	for scanner.Scan() {
		snaps = append(snaps, scanner.Text())
	}

	return snaps, nil
}

func (c *ZfsConn) RemoveSnapshot(label string) error {
	_, err := runcmd(c.sudoer, "zfs", "destroy", fmt.Sprintf("%s@%s", c.pool, label))
	return err
}

func (c *ZfsConn) Rollback(label string) error {
	_, err := runcmd(c.sudoer, "zfs", "rollback", fmt.Sprintf("%s@%s", c.pool, label))
	return err
}

func runcmd(sudoer bool, args ...string) ([]byte, error) {
	cmd := append([]string{"zfs"}, args...)
	if sudoer {
		cmd = append([]string{"sudo", "-n"}, cmd...)
	}
	glog.V(4).Infof("Executing: %v", cmd)
	return exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
}

func (d *ZfsDriver) volumeExists(v string) (bool, error) {
	out, _ := exec.Command("sudo", "zfs", "list").CombinedOutput()
	scanner := bufio.NewScanner(bytes.NewBuffer(out))
	for scanner.Scan() {
		matched, _ := regexp.MatchString(v, scanner.Text())
		if matched {
			return true, nil
		}
	}

	return false, nil
}
