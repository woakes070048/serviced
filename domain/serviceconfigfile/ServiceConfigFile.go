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

package serviceconfigfile

import (
	"time"

	"github.com/control-center/serviced/domain/servicedefinition"
	"github.com/control-center/serviced/utils"
)

// ServiceConfig is used to store and track service config files that have been modified
type ServiceConfig struct {
	ID          string                         // guid of the configuration
	ServiceID   string                         // service that owns the configuration
	ConfigFiles []servicedefinition.ConfigFile // config files for that service
	Updated     time.Time                      // datetime when the configuration was updated
	Commit      string                         // optional commit message to describe the change made to the configuration
}

// New instantiates a new ServiceConfig
func New(serviceID, commit string, conf []servicedefinition.ConfigFile) (*ServiceConfig, error) {
	uuid, err := utils.NewUUID()
	if err != nil {
		return nil, err
	}

	svcconf := &ServiceConfig{ID: uuid, ServiceID: serviceID, ConfigFiles: conf, Updated: time.Now(), Commit: commit}
	if err := svcconf.ValidEntity(); err != nil {
		return nil, err
	}

	return svcconf, nil
}
