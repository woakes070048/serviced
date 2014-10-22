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

package dfs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/control-center/serviced/commons"
	"github.com/control-center/serviced/commons/docker"
	"github.com/control-center/serviced/dao"
	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/service"
	"github.com/zenoss/glog"
)

const timeFormat = "20060102-150405"

// Snapshot takes a snapshot of the dfs as well as the docker images for the
// given service ID
func (dfs *DistributedFilesystem) Snapshot(tenantID string) (string, error) {
	// Get the tenant (parent) service
	var tenant service.Service
	if err := dfs.client.GetService(tenantID, &tenant); err != nil {
		glog.Errorf("Could not get service %s: %s", tenantID, err)
		return "", err
	}

	// Get all the child services for that tenant
	var svcs []service.Service
	if err := dfs.client.GetServices(dao.ServiceRequest{TenantID: tenantID}, &svcs); err != nil {
		glog.Errorf("Could not get child services for %s: %s", tenantID, err)
		return "", err
	}

	// Pause all running services
	defer dfs.client.ResumeServices(nil, nil) // Pause doesn't restart services if it fails
	if err := dfs.client.PauseServices(dfs.timeout, nil); err != nil {
		glog.Errorf("Could not pause all services: %s", err)
		return "", err
	}

	// create the snapshot
	snapshotVolume, err := dfs.GetVolume(tenant.ID)
	if err != nil {
		glog.Errorf("Could not acquire the snapshot volume for %s (%s): %s", tenant.Name, tenant.ID, err)
		return "", err
	}

	tagID := time.Now().UTC().Format(timeFormat)
	label := fmt.Sprintf("%s_%s", tenantID, tagID)

	// add the snapshot to the volume
	if err := snapshotVolume.Snapshot(label); err != nil {
		glog.Errorf("Could not snapshot service %s (%s): %s", tenant.Name, tenant.ID, err)
		return "", err
	}

	// tag all of the images
	if err := tag(tenantID, DockerLatest, tagID); err != nil {
		glog.Errorf("Could not tag new snapshot for %s (%s): %s", tenant.Name, tenant.ID, err)
		return "", err
	}

	// dump the service definitions
	if err := exportJSON(filepath.Join(snapshotVolume.SnapshotPath(label), serviceJSON), svcs); err != nil {
		glog.Errorf("Could not export existing services at %s: %s", snapshotVolume.SnapshotPath(label), err)
		return "", err
	}

	return label, nil
}

// Rollback rolls back the dfs and docker images to the state of a given snapshot
// It is implied that all service instances have stopped
func (dfs *DistributedFilesystem) Rollback(snapshotID string) error {
	tenantID, timestamp, err := parseLabel(snapshotID)
	if err != nil {
		glog.Errorf("Could not rollback snapshot %s: %s", snapshotID, err)
		return err
	}

	// check the snapshot
	var tenant service.Service
	if err := dfs.client.GetService(tenantID, &tenant); err != nil {
		glog.Errorf("Could not find service %s: %s", tenantID, err)
		return err
	}

	snapshotVolume, err := dfs.GetVolume(tenant.ID)
	if err != nil {
		glog.Errorf("Could not find volume for service %s: %s", tenantID, err)
		return err
	}

	// rollback the dfs
	glog.V(0).Infof("Performing rollback for %s (%s) using %s", tenant.Name, tenant.ID, snapshotID)
	if err := snapshotVolume.Rollback(snapshotID); err != nil {
		glog.Errorf("Error while trying to roll back to %s: %s", snapshotID, err)
		return err
	}

	// restore the tags
	glog.V(0).Infof("Restoring image tags for %s", snapshotID)
	if err := tag(tenantID, timestamp, DockerLatest); err != nil {
		glog.Errorf("Could not restore snapshot tags for %s (%s): %s", tenant.Name, tenant.ID, err)
		return err
	}

	// restore services
	var restore []*service.Service
	if err := importJSON(filepath.Join(snapshotVolume.SnapshotPath(snapshotID), serviceJSON), &restore); err != nil {
		glog.Errorf("Could not acquire services from %s: %s", snapshotID, err)
		return err
	}

	if err := dfs.restoreServices(restore); err != nil {
		glog.Errorf("Could not restore services from %s: %s", snapshotID, err)
		return err
	}

	return nil
}

// ListSnapshots lists all the snapshots for a particular tenant
func (dfs *DistributedFilesystem) ListSnapshots(tenantID string) ([]string, error) {
	// Get the tenant (parent) service
	var tenant service.Service
	if err := dfs.client.GetService(tenantID, &tenant); err != nil {
		glog.Errorf("Could not get service %s: %s", tenantID, err)
		return nil, err
	}

	snapshotVolume, err := dfs.GetVolume(tenant.ID)
	if err != nil {
		glog.Errorf("Could not find volume for service %s (%s): %s", tenant.Name, tenant.ID, err)
		return nil, err
	}

	return snapshotVolume.Snapshots()
}

// DeleteSnapshot deletes an existing snapshot as identified by its snapshotID
func (dfs *DistributedFilesystem) DeleteSnapshot(snapshotID string) error {
	tenantID, timestamp, err := parseLabel(snapshotID)
	if err != nil {
		glog.Errorf("Could not parse snapshot ID %s: %s", snapshotID, err)
		return err
	}

	var tenant service.Service
	if err := dfs.client.GetService(tenantID, &tenant); err != nil {
		glog.Errorf("Service not found %s: %s", tenantID, err)
		return err
	}

	snapshotVolume, err := dfs.GetVolume(tenant.ID)
	if err != nil {
		glog.Errorf("Could not find the volume for service %s (%s): %s", tenant.Name, tenant.ID, err)
		return err
	}

	// delete the snapshot
	if err := snapshotVolume.RemoveSnapshot(snapshotID); err != nil {
		glog.Errorf("Could not delete snapshot %s: %s", snapshotID, err)
		return err
	}

	// update the tags
	images, err := findImages(tenantID, timestamp)
	if err != nil {
		glog.Errorf("Could not find images for snapshot %s: %s", snapshotID, err)
		return err
	}

	for _, image := range images {
		imageID := image.ID
		imageID.Tag = timestamp
		img, err := docker.FindImage(imageID.String(), false)
		if err != nil {
			glog.Errorf("Could not remove tag from image %s: %s", imageID, err)
			continue
		}
		img.Delete()
	}

	return nil
}

// DeleteSnapshots deletes all snapshots relating to a particular tenantID
func (dfs *DistributedFilesystem) DeleteSnapshots(tenantID string) error {
	// delete the snapshot subvolume
	snapshotVolume, err := dfs.GetVolume(tenantID)
	if err != nil {
		glog.Errorf("Could not find the volume for service %s (%s): %s")
		return err
	}
	if err := snapshotVolume.Unmount(); err != nil {
		glog.Errorf("Could not unmount volume for service %s: %s", tenantID, err)
		return err
	}

	// delete images for that tenantID
	images, err := searchImagesByTenantID(tenantID)
	if err != nil {
		glog.Errorf("Error looking up images for %s: %s", tenantID)
		return err
	}

	for _, image := range images {
		if err := image.Delete(); err != nil {
			glog.Warningf("Could not delete image %s (%s): %s", image.ID, image.UUID, err)
		}
	}

	return nil
}

func (dfs *DistributedFilesystem) restoreServices(svcs []*service.Service) error {
	// get the resource pools
	pools, err := dfs.facade.GetResourcePools(datastore.Get())
	if err != nil {
		return err
	}
	poolMap := make(map[string]struct{})
	for _, pool := range pools {
		poolMap[pool.ID] = struct{}{}
	}

	// map services to parent
	serviceTree := make(map[string][]service.Service)
	for _, svc := range svcs {
		serviceTree[svc.ParentServiceID] = append(serviceTree[svc.ParentServiceID], *svc)
	}

	// map service id to service
	current, err := dfs.facade.GetServices(datastore.Get(), dao.ServiceRequest{})
	if err != nil {
		glog.Errorf("Could not get services: %s", err)
		return err
	}

	currentServices := make(map[string]*service.Service)
	for _, svc := range current {
		currentServices[svc.ID] = &svc
	}

	// updates all of the services
	var traverse func(parentID string) error
	traverse = func(parentID string) error {
		for _, svc := range serviceTree[parentID] {
			serviceID := svc.ID
			svc.DatabaseVersion = 0
			svc.DesiredState = service.SVCStop
			svc.ParentServiceID = parentID

			// update the image
			if svc.ImageID != "" {
				image, err := commons.ParseImageID(svc.ImageID)
				if err != nil {
					glog.Errorf("Invalid image %s for %s (%s): %s", svc.ImageID, svc.Name, svc.ID)
				}
				image.Host = dfs.dockerHost
				image.Port = dfs.dockerPort
				svc.ImageID = image.BaseName()
			}

			// check the pool
			if _, ok := poolMap[svc.PoolID]; !ok {
				glog.Warningf("Could not find pool %s for %s (%s). Setting pool to default.", svc.PoolID, svc.Name, svc.ID)
				svc.PoolID = "default"
			}

			if _, ok := currentServices[serviceID]; ok {
				if err := dfs.facade.UpdateService(datastore.Get(), svc); err != nil {
					glog.Errorf("Could not update service %s: %s", svc.ID, err)
					return err
				}
				delete(currentServices, serviceID)
			} else {
				if err := dfs.facade.AddService(datastore.Get(), svc); err != nil {
					glog.Errorf("Could not add service %s: %s", serviceID, err)
					return err
				}

				/*
					// TODO: enable this to generate a new service ID, instead of recycling
					// the old one
					svc.ID = ""
					newServiceID, err = dfs.facade.AddService(svc)
					if err != nil {
						glog.Errorf("Could not add service %s: %s", serviceID, err)
						return err
					}

					// Update the service
					serviceTree[newServiceID] = serviceTree[serviceID]
					delete(serviceTree, serviceID)
					serviceID = newServiceID
				*/
			}

			if err := traverse(serviceID); err != nil {
				return err
			}
		}
		return nil
	}

	if err := traverse(""); err != nil {
		glog.Errorf("Error while rolling back services: %s", err)
		return err
	}

	/*
		// TODO: enable this if we want to delete any non-matching services
		for serviceID := range currentServices {
			if err := dfs.facade.RemoveService(serviceID); err != nil {
				glog.Errorf("Could not remove service %s: %s", serviceID, err)
				return err
			}
		}
	*/

	return nil
}

func NewLabel(tenantID string) string {
	return fmt.Sprintf("%s_%s", tenantID, time.Now().UTC().Format(timeFormat))
}

func parseLabel(snapshotID string) (string, string, error) {
	parts := strings.SplitN(snapshotID, "_", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("malformed label")
	}
	return parts[0], parts[1], nil
}

func exportJSON(filename string, v interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		glog.Errorf("Could not create file at %s: %v", filename, err)
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			glog.Warningf("Error while closing file %s: %s", filename, err)
		}
	}()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(v); err != nil {
		glog.Errorf("Could not write JSON data to %s: %s", filename, err)
		return err
	}

	return nil
}

func importJSON(filename string, v interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		glog.Errorf("Could not open file at %s: %v", filename, err)
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			glog.Warningf("Error while closing file %s: %s", filename, err)
		}
	}()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(v); err != nil {
		glog.Errorf("Could not read JSON data from %s: %s", filename, err)
		return err
	}

	return nil
}
