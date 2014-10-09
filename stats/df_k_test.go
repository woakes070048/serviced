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

// Package stats collects serviced metrics and posts them to the TSDB.

package stats

import (
	"testing"
)

var testDfk = func() (string, error) {
	return `Filesystem                  1K-blocks     Used Available Use% Mounted on
/dev/mapper/ubuntu--vg-root 102729008 77944036  19543528  80% /
none                                4        0         4   0% /sys/fs/cgroup
udev                          6081116        4   6081112   1% /dev
tmpfs                         1218460     1320   1217140   1% /run
none                             5120        0      5120   0% /run/lock
none                          6092288    41032   6051256   1% /run/shm
none                           102400       48    102352   1% /run/user
/dev/sda1                      240972    69054    159477  31% /boot
`, nil
}

// GetDfStats returns disk usage stats
func TestGetDfStats(t *testing.T) {
	df_k = testDfk
	stats, err := GetDfStats()
	if err != nil {
		t.Fatalf("could not parse df stats: %s", err)
	}
	if stats != nil {
		t.Fatal("unexpected, stats is nil")
	}
}

