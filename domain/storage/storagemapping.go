// Copyright 2015 The Serviced Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"github.com/control-center/serviced/datastore/elastic"
	"github.com/zenoss/glog"
)

const mappingString = `
{
    "storage": {
        "properties": {
            "ID":          {"type":"string", "index":"not_analyzed"},
            "OSDID":       {"type":"integer", "index":"not_analyzed"},
			"HostID":      {"type":"string",  "index":"not_analyzed"},
            "DataPath":    {"type":"string",  "index":"not_analyzed"},
			"JournalPath": {"type":"string",  "index":"not_analyzed"},
            "Weight":      {"type":"float",   "index":"not_analyzed"}
        }
    }
}
`

var MAPPING, mappingError = elastic.NewMapping(mappingString)

func init() {
	if mappingError != nil {
		glog.Fatalf("error creating service mapping: %v", mappingError)
	}
}
