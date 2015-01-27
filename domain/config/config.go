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

package config

import (
	"fmt"
	"sort"
	"time"

	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/servicedefinition"
	"github.com/control-center/serviced/utils"
)

// SortFiles is a sorting type for ConfigFile. It sorts by ID first
// and then by filename
type SortFiles []servicedefinition.ConfigFile

func (s SortFiles) Len() int { return len(s) }

func (s SortFiles) Less(a, b int) bool {
	if s[a].ID != "" && s[b].ID != "" {
		return s[a].ID < s[b].ID
	} else if s[a].ID != "" {
		return true
	} else if s[b].ID != "" {
		return false
	} else {
		return s[a].Filename < s[b].Filename
	}
}

func (s SortFiles) Swap(a, b int) { s[a], s[b] = s[b], s[a] }

func (s SortFiles) Sort() {
	sort.Sort(s)
}

func validate(filemap map[string]servicedefinition.ConfigFile) ([]servicedefinition.ConfigFile, error) {
	var (
		byID   = make(map[string]struct{})
		byName = make(map[string]struct{})
		files  = make([]servicedefinition.ConfigFile, 0)
	)

	for tag, file := range filemap {
		if file.ID != "" {
			if _, ok := byID[file.ID]; !ok {
				byID[file.ID] = struct{}{}
			} else {
				return nil, fmt.Errorf("duplicate id %s", file.ID)
			}
		}
		if file.Filename != "" {
			if _, ok := byName[file.Filename]; !ok {
				byName[file.Filename] = struct{}{}
			} else {
				return nil, fmt.Errorf("filename cannot be empty %s (%s)", file.ID, tag)
			}
		}
		files = append(files, file)
	}

	return files, nil
}

// ServiceConfigHistory stores config file data for a specific service with a
// given tenantID and service path
type ServiceConfigHistory struct {
	ID            string
	TenantID      string
	ServicePath   string
	CreatedAt     time.Time
	CommitMessage string
	ConfigFiles   map[string]servicedefinition.ConfigFile
	RefID         string // ID of the history record that this is being restored from
	datastore.VersionedEntity
}

// NewRecord creates a new ServiceConfigHistory record
func NewRecord(tenantID, servicePath string, configFiles map[string]servicedefinition.ConfigFile) (*ServiceConfigHistory, error) {
	uuid, err := utils.NewUUID36()
	if err != nil {
		return nil, err
	}

	return &ServiceConfigHistory{
		ID:          uuid,
		TenantID:    tenantID,
		ServicePath: servicePath,
		CreatedAt:   time.Now(),
		ConfigFiles: configFiles,
	}, nil
}

// Generate creates a new ServiceConfigHistoryRecord with validation
// and file parsing for a map of config data
func Generate(tenantID, servicePath string, filemap map[string]servicedefinition.ConfigFile) (*ServiceConfigHistory, error) {
	files, err := validate(filemap)
	if err != nil {
		return nil, err
	}

	// TODO: do we need to regenerate IDs for all records?
	filemap = make(map[string]servicedefinition.ConfigFile)
	for _, f := range files {
		var err error
		if f.ID, err = utils.NewUUID36(); err != nil {
			return nil, err
		}
		f.CreatedAt = time.Now()
		f.UpdatedAt = f.CreatedAt
		filemap[f.ID] = f
	}
	return NewRecord(tenantID, servicePath, filemap)
}

// Generate compares the current record against the filemap passed in
// and creates a record if a diff is discovered
func (record *ServiceConfigHistory) Generate(filemap map[string]servicedefinition.ConfigFile, newRecord *ServiceConfigHistory) (bool, error) {
	files, err := validate(filemap)
	if err != nil {
		return false, err
	}
	SortFiles(files).Sort()

	// we need to compare against config ID and filename (if ID is not
	// available)
	byName := make(map[string]string)
	for _, file := range record.ConfigFiles {
		byName[file.Filename] = file.ID
	}

	isnew := false
	filemap = make(map[string]servicedefinition.ConfigFile)
	for _, file := range files {
		var f *servicedefinition.ConfigFile

		// Get the config file (if it exists)
		if c, ok := record.ConfigFiles[file.ID]; ok {
			f = &c
		} else if id, ok := byName[file.Filename]; ok {
			c := record.ConfigFiles[id]
			f = &c
		}

		if f != nil {
			// delete the filename record
			delete(byName, f.Filename)
			// check for diffs
			if f.Equals(file) {
				filemap[f.ID] = *f
			} else {
				isnew = true
				file.CreatedAt = f.CreatedAt
				file.UpdatedAt = time.Now()
				file.RefID = record.RefID
				filemap[f.ID] = file
			}
		} else {
			// create a new config file
			// TODO: do we need to regenerate id for all records?
			isnew = true
			var err error
			if file.ID, err = utils.NewUUID36(); err != nil {
				return false, err
			}
			file.CreatedAt = time.Now()
			file.UpdatedAt = file.CreatedAt
			filemap[file.ID] = file
		}
	}

	if isnew {
		r, err := NewRecord(record.TenantID, record.ServicePath, filemap)
		if err != nil {
			return false, err
		}
		*newRecord = *r
	}

	return isnew, nil
}
