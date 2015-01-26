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

package facade

import (
	"fmt"
	"path"

	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/config"
	"github.com/control-center/serviced/domain/service"
	"github.com/control-center/serviced/domain/serviceconfigfile"
	"github.com/control-center/serviced/domain/servicedefinition"

	"github.com/zenoss/glog"
)

// AddConfig creates a configuration record for a new service
func (f *Facade) AddConfig(ctx datastore.Context, svc *service.Service) error {
	// get the path to the parent and append the service
	tenantID, servicePath, err := f.getServicePath(ctx, svc.ParentServiceID)
	if err != nil {
		glog.Errorf("Could not acquire service path and tenantID for %s: %s", svc.ParentServiceID, err)
		return err
	}
	servicePath = path.Join(servicePath, svc.Name)

	store := config.NewStore()
	var record *config.ServiceConfigHistory // the new record

	// check if there are records available at this path
	currentRecord, err := store.GetServiceConfig(ctx, tenantID, servicePath)
	if err != nil {
		glog.Errorf("Could not check config records for service %s (%s): %s", svc.Name, servicePath, err)
		return err
	} else if currentRecord != nil {
		// TODO: is this the correct way to handle this use case?

		// get the original config record
		oldestRecord, err := store.GetOriginalServiceConfig(ctx, tenantID, servicePath)
		if err != nil {
			glog.Errorf("Could not look up the original config record for service %s (%s): %s", svc.Name, servicePath, err)
			return err
		} else if oldestRecord == nil {
			// this should never happen
			err := fmt.Errorf("no config records found")
			glog.Errorf("Could not look up the original config record for service %s (%s): %s", svc.Name, servicePath, err)
			return err
		}
		svc.OriginalConfigs = oldestRecord.ConfigFiles

		// generate the next record
		record := &config.ServiceConfigHistory{}
		if ok, err := currentRecord.Generate(svc.ConfigFiles, record); err != nil {
			glog.Errorf("Could not generate new record for %s (%s): %s", svc.Name, servicePath, err)
			return err
		} else if !ok {
			// nothing changed, so we are done!
			glog.Infof("No new config changes found for %s (%s)", servicePath, currentRecord.ID)
			svc.ConfigFiles = currentRecord.ConfigFiles
			return nil
		}
		// set the commit message
		record.CommitMessage = "service add"
	} else {
		// create a new record
		record, err := config.Generate(tenantID, servicePath, svc.ConfigFiles)
		if err != nil {
			glog.Errorf("Could not build new record for %s (%s): %s", svc.Name, servicePath, err)
			return err
		}
		// set the commit message
		record.CommitMessage = "initial revision"
		svc.OriginalConfigs = record.ConfigFiles
	}

	svc.ConfigFiles = record.ConfigFiles
	return store.Put(ctx, record)
}

// UpdateConfig creates a new configuration record for an existing service
func (f *Facade) UpdateConfig(ctx datastore.Context, svc *service.Service) error {
	return f.CommitConfig(ctx, svc, "service update")
}

// CommitConfig creates a new configuration record for an existing service
func (f *Facade) CommitConfig(ctx datastore.Context, svc *service.Service, commitMessage string) error {
	// get the path to the service
	tenantID, servicePath, err := f.getServicePath(ctx, svc.ID)
	if err != nil {
		glog.Errorf("Could not acquire service path and tenantID for %s: %s", svc.ID, err)
		return err
	}

	store := config.NewStore()

	// set the original config, as they are immutable
	oldestRecord, err := store.GetOriginalServiceConfig(ctx, tenantID, servicePath)
	if err != nil {
		glog.Errorf("Could not look up the original config record for service %s (%s): %s", svc.Name, servicePath, err)
		return err
	} else if oldestRecord == nil {
		// this should never happen
		err := fmt.Errorf("no config records found")
		glog.Errorf("Could not look up the original config record for service %s (%s): %s", svc.Name, servicePath, err)
		return err
	}
	svc.OriginalConfigs = oldestRecord.ConfigFiles

	// get the latest record
	currentRecord, err := store.GetServiceConfig(ctx, tenantID, servicePath)
	if err != nil {
		glog.Errorf("Could not check config records for service %s (%s): %s", svc.ID, servicePath, err)
		return err
	} else if currentRecord == nil {
		// this should never happen
		err := fmt.Errorf("no config records found")
		glog.Errorf("Could not check config records for service %s (%s): %s", svc.ID, servicePath, err)
		return err
	}

	// generate the next record
	var record config.ServiceConfigHistory
	if ok, err := currentRecord.Generate(svc.ConfigFiles, &record); err != nil {
		glog.Errorf("Could not generate new record for %s (%s): %s", svc.ID, servicePath, err)
		return err
	} else if !ok {
		// nothing changed, so we are done!
		glog.V(2).Infof("No new config changes found for %s (%s)", servicePath, currentRecord.ID)
		svc.ConfigFiles = currentRecord.ConfigFiles
		return nil
	} else {
		glog.V(2).Infof("Writing new config record %s (%s)", servicePath, record.ID)
		record.CommitMessage = commitMessage
		svc.ConfigFiles = record.ConfigFiles
		return store.Put(ctx, &record)
	}
}

// RestoreConfig restores a service's configuration based on a specific history record
func (f *Facade) RestoreConfig(ctx datastore.Context, serviceID, historyID string) error {
	// get the path to the parent and append the service
	tenantID, servicePath, err := f.getServicePath(ctx, serviceID)
	if err != nil {
		glog.Errorf("Could not acquire service path and tenantID for %s: %s", serviceID, err)
		return err
	}

	store := config.NewStore()

	// find the record to restore
	record, err := store.Get(ctx, historyID)
	if err != nil {
		glog.Errorf("Error while retrieving record %s: %s", historyID, err)
		return err
	}

	// check if the record is not already the latest
	currentRecord, err := store.GetServiceConfig(ctx, tenantID, servicePath)
	if err != nil {
		glog.Errorf("Could not check config records for service %s (%s): %s", serviceID, servicePath, err)
		return err
	} else if currentRecord == nil {
		err := fmt.Errorf("no config records found")
		glog.Errorf("Could not check config records for service %s (%s): %s", serviceID, servicePath, err)
		return err
	} else if currentRecord.ID == record.ID {
		glog.Infof("Record %s is the latest", record.ID)
		return nil
	}

	// make the new history record
	var newRecord config.ServiceConfigHistory
	if ok, err := currentRecord.Generate(record.ConfigFiles, &newRecord); err != nil {
		glog.Errorf("Could not generate config record for %s (%s): %s", serviceID, servicePath, err)
		return err
	} else if !ok {
		glog.Infof("Record %s is the latest", record.ID)
		return nil
	}

	newRecord.RefID = record.ID
	newRecord.CommitMessage = fmt.Sprintf("[restored] %s", record.CommitMessage)
	return store.Put(ctx, &newRecord)
}

// GetConfig imports the configuration files for a service from its history.
// If the record doesn't exist, then it will try to import the file from
// deprecated resources.
func (f *Facade) GetConfig(ctx datastore.Context, svc *service.Service) error {
	// get the path to the service
	tenantID, servicePath, err := f.getServicePath(ctx, svc.ID)
	if err != nil {
		glog.Errorf("Could not acquire service path and tenantID for %s: %s", svc.ID, err)
		return err
	}

	store := config.NewStore()
	// get the oldest history record
	oldestRecord, err := store.GetOriginalServiceConfig(ctx, tenantID, servicePath)
	if err != nil {
		glog.Errorf("Could not get original config for %s (%s): %s", svc.ID, servicePath, err)
		return err
	} else if oldestRecord == nil {
		// if the record is nil, then we need to migrate the configuration
		glog.Infof("Migrating original config files for %s (%s)", svc.ID, servicePath)
		oldestRecord, err = config.Generate(tenantID, servicePath, svc.OriginalConfigs)
		if err != nil {
			glog.Errorf("Could not create original history record for %s: %s", svc.ID, err)
			return err
		}

		// Assume the create/update datetime of the record reflects the
		// datetime that the service was created
		for key, conf := range oldestRecord.ConfigFiles {
			conf.CreatedAt = svc.CreatedAt
			conf.UpdatedAt = svc.CreatedAt
			oldestRecord.ConfigFiles[key] = conf
		}
		oldestRecord.CommitMessage = "[migrated] initial revision"

		// Write the record to the database
		if err := store.Put(ctx, oldestRecord); err != nil {
			glog.Errorf("Could not write original history record for %s (%s): %s", svc.ID, servicePath, err)
			return err
		}
	}
	svc.OriginalConfigs = oldestRecord.ConfigFiles

	// get the latest history record
	currentRecord, err := store.GetServiceConfig(ctx, tenantID, servicePath)
	if err != nil {
		glog.Errorf("Could not get current config for %s (%s): %s", svc.ID, servicePath, err)
		return err
	} else if currentRecord == nil {
		// if the record is nil, something is wrong because every service must
		// have at least record at this point
		err := fmt.Errorf("no config records found")
		glog.Errorf("Could not find current config for %s (%s): %s", svc.ID, servicePath, err)
		return err
	} else if currentRecord.ID == oldestRecord.ID {
		// there is one record stored in history, so we need to make sure we do not
		// have any other records saved in the deprecated store
		cstore := serviceconfigfile.NewStore()
		sconfs, err := cstore.GetConfigFiles(ctx, tenantID, servicePath)
		if err != nil {
			glog.Errorf("Could not get deprecated config for %s (%s): %s", svc.ID, servicePath, err)
			return err
		}
		files := make(map[string]servicedefinition.ConfigFile)
		for _, sconf := range sconfs {
			files[sconf.ID] = sconf.ConfFile
		}

		var r config.ServiceConfigHistory
		if ok, err := oldestRecord.Generate(files, &r); err != nil {
			glog.Errorf("Could not migrate deprecate config for %s (%s): %s", svc.ID, servicePath, err)
			return err
		} else if ok {
			currentRecord = &r

			// Assume the update datetime of the record reflects the datetime
			// that the service was last updated (best guess)
			for key, conf := range currentRecord.ConfigFiles {
				conf.UpdatedAt = svc.UpdatedAt
				currentRecord.ConfigFiles[key] = conf
			}
			currentRecord.CommitMessage = "[migrated] v1.0"

			// Write the record to the database
			if err := store.Put(ctx, currentRecord); err != nil {
				glog.Errorf("Could not write the current history record for %s (%s): %s", svc.ID, servicePath, err)
				return err
			}
		}
	}
	svc.ConfigFiles = currentRecord.ConfigFiles

	return nil
}

// getServicePath returns the service path by tracking its parent and also
// returns the tenantID
func (f *Facade) getServicePath(ctx datastore.Context, serviceID string) (string, string, error) {
	store := f.serviceStore
	svc, err := store.Get(ctx, serviceID)
	if err != nil {
		return "", "", err
	}
	if svc.ParentServiceID == "" {
		return svc.ID, "/" + svc.Name, nil
	}
	tenantID, svcPath, err := f.getServicePath(ctx, svc.ParentServiceID)
	if err != nil {
		return "", "", err
	}
	return tenantID, path.Join(svcPath, svc.Name), nil
}
