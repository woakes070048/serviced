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

package utils

//#include <stdlib.h>
//extern int fd_lock(int fd, char* filepath);
//extern int fd_unlock(int fd, char* filepath);
import "C"
import (
	"github.com/zenoss/glog"

	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// LockFile opens and locks the file at the given path - it will block and wait for unlock
func LockFile(filelockpath string) (*os.File, error) {

WAIT_FOR_LOCK:
	for {
		glog.V(2).Infof("locking file: %s", filelockpath)
		fp, err := os.OpenFile(filelockpath, syscall.O_RDWR|syscall.O_CREAT|syscall.O_EXCL, 0600)
		if err != nil {
			if !strings.Contains(err.Error(), syscall.EEXIST.Error()) {
				return nil, err
			}
		} else {
			glog.V(2).Infof("locked file: %s", filelockpath)
			return fp, nil
		}

		glog.Infof("waiting to lock file: %s", filelockpath)
		select {
		case <-time.After(1 * time.Second):
			continue WAIT_FOR_LOCK
		}
	}

	return nil, fmt.Errorf("timed out waiting to lock file %s", filelockpath)

	// other failed attempts to lock file using fcntl/flock/...
	fp, err := os.OpenFile(filelockpath, syscall.O_RDWR|syscall.O_CREAT, 0600)
	if err != nil {
		return nil, err
	}

	glog.V(2).Infof("locking file: %s", filelockpath)
	if true {
		// flock tested and works on ubuntu: ext4, btrfs, NFS (same host)
		// BEWARE: flock does not work when two hosts are NFS locking
		// TODO: test on centos: tmpfs, ext4, btrfs, NFS
		if err := syscall.Flock(int(fp.Fd()), syscall.LOCK_EX); err != nil {
			return nil, err
		}
	} else if false {
		// FIXME: this is not working from GO, but works from C
		var flp = C.CString(filelockpath)
		defer C.free(unsafe.Pointer(flp))
		rc := C.fd_lock(C.int(fp.Fd()), flp)
		if 0 != rc {
			return nil, fmt.Errorf("unable to lock file %s", filelockpath)
		}
	} else if false {
		// FIXME: fcntl is not working
		ft := syscall.Flock_t{
			Type:   syscall.F_WRLCK,         //Type of lock: F_RDLCK, F_WRLCK, F_UNLCK
			Whence: int16(os.SEEK_SET),      // How to interpret l_start: SEEK_SET, SEEK_CUR, SEEK_END
			Start:  0,                       // Starting offset for lock
			Len:    0,                       // Number of bytes to lock
			Pid:    int32(syscall.Getpid()), // PID of process blocking our lock (F_GETLK only)
		}
		glog.V(2).Infof("syscall.Flock_t %#v", ft)

		if err := syscall.FcntlFlock(fp.Fd(), syscall.F_SETLKW /* F_GETLK, F_SETLK, F_SETLKW */, &ft); err != nil {
			return nil, err
		}
	} else {
		// This type matches C's "struct flock" defined in /usr/include/x86_64-linux-gnu/bits/fcntl.h
		// TODO: move this into the standard syscall package.
		k := struct {
			Type   uint32
			Whence uint32
			Start  uint64
			Len    uint64
			Pid    uint32
		}{
			Type:   syscall.F_WRLCK,
			Whence: uint32(os.SEEK_SET),
			Start:  0,
			Len:    0, // 0 means to lock the entire file.
			Pid:    uint32(os.Getpid()),
		}

		glog.V(2).Infof("mimic /usr/include/x86_64-linux-gnu/bits/fcntl.h syscall.Flock_t %#v", k)
		_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fp.Fd(), uintptr(syscall.F_SETLK), uintptr(unsafe.Pointer(&k)))
		if errno != 0 {
			fp.Close()
			return nil, fmt.Errorf("syscall.Syscall error: %s", errno)
		}
	}

	glog.V(2).Infof("locked  file: %s", filelockpath)
	return fp, nil
}