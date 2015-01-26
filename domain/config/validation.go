// Copyright 2014 The Serviced Authors.
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

package config

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/control-center/serviced/validation"
)

func (record *ServiceConfigHistory) ValidEntity() error {
	vErr := validation.NewValidationError()
	vErr.Add(validation.NotEmpty("ID", record.ID))
	vErr.Add(validation.NotEmpty("TenantID", record.TenantID))
	vErr.Add(validation.NotEmpty("ServicePath", record.ServicePath))

	record.validFiles(vErr)
	if vErr.HasError() {
		return vErr
	}
	return nil
}

// validFiles ensures all the config files are valid and unique
func (record *ServiceConfigHistory) validFiles(vErr *validation.ValidationError) {
	var (
		byName = make(map[string]sync.Once)
	)

	for tag, config := range record.ConfigFiles {
		if tag != config.ID {
			vErr.Add(fmt.Errorf("mismatch tag (%s) and id (%s)", tag, config.ID))
		}

		filename := filepath.Clean(config.Filename)
		if once, ok := byName[filename]; ok {
			once.Do(func() {
				vErr.Add(fmt.Errorf("found multiple files at %s", filename))
			})
		} else {
			byName[filename] = sync.Once{}
		}
	}
}
