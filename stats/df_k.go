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
	"strconv"
	"strings"
	"os/exec"
	"time"
)

var df_k = func() (output string, err error) {
	out, err := exec.Command("df", "-k").Output()
	if err != nil {
		return "", err
	}
	return string(out), err
}

// GetDfStats returns disk usage stats
func GetDfStats() (stats []Sample, err error) {
	output, err := df_k()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	lines := strings.Split(output, "\n")
	stats = make([]Sample, 0)
	for i := 1; i < len(lines); i++ {
		fields := strings.Fields(lines[i])
		if len(fields) < 6 {
			continue
		}
		vals := make([]int, 3)
		for i := 0; i < 3; i++ {
			val, err := strconv.Atoi(fields[i+1])
			if err != nil {
				return nil, err
			}
			vals[i] = val
		}
		total := 0
		if strings.HasSuffix(fields[4], "%") && len(fields[4]) >= 2 {
			total, err = strconv.Atoi(fields[4][:len(fields[4])-1])
		} else {
			total, err = strconv.Atoi(fields[4])
		}
		if err != nil {
			return nil, err
		}
	
		stats = append(stats, Sample{
			Metric: "disk.total",
			Value: strconv.Itoa(vals[0] * 1024),
			Timestamp: now,
			Tags: map[string]string{"filesystem": fields[0]},
		})
		stats = append(stats, Sample{
			Metric: "disk.used",
			Value: strconv.Itoa(vals[1] * 1024),
			Timestamp: now,
			Tags: map[string]string{"filesystem": fields[0]},
		})
		stats = append(stats, Sample{
			Metric: "disk.available",
			Value: strconv.Itoa(vals[2] * 1024),
			Timestamp: now,
			Tags: map[string]string{"filesystem": fields[0]},
		})
		stats = append(stats, Sample{
			Metric: "disk.used.percent",
			Value: strconv.Itoa(total),
			Timestamp: now,
			Tags: map[string]string{"filesystem": fields[0]},
		})
	}
	return stats, nil
}

