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
	"time"

	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/servicedefinition"
	"github.com/control-center/serviced/utils"
)

type ServiceConfigHistory struct {
	ID            string
	TenantID      string
	ServicePath   string
	CreatedAt     time.Time
	CommitMessage string
	ConfigFiles   map[string]servicedefinition.ConfigFile
	datastore.VersionedEntity
}

func NewRecord(tenantID, servicePath, commitMessage string, configFiles map[string]servicedefinition.ConfigFile) (*ServiceConfigHistory, error) {
	uuid, err := utils.NewUUID36()
	if err != nil {
		return nil, err
	}

	return &ServiceConfigHistory{
		ID:            uuid,
		TenantID:      tenantID,
		ServicePath:   servicePath,
		CreatedAt:     time.Now(),
		CommitMessage: commitMessage,
		ConfigFiles:   configFiles,
	}, nil
}
