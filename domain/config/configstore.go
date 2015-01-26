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
	"errors"
	"strings"

	"github.com/control-center/serviced/datastore"

	"github.com/zenoss/elastigo/search"
)

func NewStore() *Store {
	return &Store{}
}

// Store type for interacting with ServiceConfigHistory persistent storage
type Store struct {
	ds datastore.DataStore
}

// Put adds or update a ServiceConfigHistory record
func (s *Store) Put(ctx datastore.Context, config *ServiceConfigHistory) error {
	return s.ds.Put(ctx, Key(config.ID), config)
}

// Get a ServiceConfigHistory record by id. Return ErrNoSuchEntity if not found
func (s *Store) Get(ctx datastore.Context, id string) (*ServiceConfigHistory, error) {
	var config ServiceConfigHistory
	if err := s.ds.Get(ctx, Key(id), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// Delete removes the ServiceConfigHistory record if it exists
func (s *Store) Delete(ctx datastore.Context, id string) error {
	return s.ds.Delete(ctx, Key(id))
}

// GetOriginalServiceConfig returns the oldest config for a service
func (s *Store) GetOriginalServiceConfig(ctx datastore.Context, tenantID string, servicePath string) (*ServiceConfigHistory, error) {
	if tenantID = strings.TrimSpace(tenantID); tenantID == "" {
		return nil, errors.New("empty TenantID not allowed")
	} else if servicePath = strings.TrimSpace(servicePath); servicePath == "" {
		return nil, errors.New("empty ServicePath not allowed")
	}

	search := search.Search("controlplane").Type(kind).Size("1").Sort(
		search.Sort("CreatedAt").Asc(),
	).Filter(
		"and",
		search.Filter().Terms("TenantID", tenantID),
		search.Filter().Terms("ServicePath", servicePath),
	)

	q := datastore.NewQuery(ctx)
	results, err := q.Execute(search)
	if err != nil {
		return nil, err
	}

	if configs, err := convert(results); err != nil || len(configs) == 0 {
		return nil, err
	} else {
		return &configs[0], nil
	}
}

// GetServiceConfig returns the latest config for a service
func (s *Store) GetServiceConfig(ctx datastore.Context, tenantID string, servicePath string) (*ServiceConfigHistory, error) {
	if tenantID = strings.TrimSpace(tenantID); tenantID == "" {
		return nil, errors.New("empty TenantID not allowed")
	} else if servicePath = strings.TrimSpace(servicePath); servicePath == "" {
		return nil, errors.New("empty ServicePath not allowed")
	}

	search := search.Search("controlplane").Type(kind).Size("1").Sort(
		search.Sort("CreatedAt").Desc(),
	).Filter(
		"and",
		search.Filter().Terms("TenantID", tenantID),
		search.Filter().Terms("ServicePath", servicePath),
	)

	q := datastore.NewQuery(ctx)
	results, err := q.Execute(search)
	if err != nil {
		return nil, err
	}

	if configs, err := convert(results); err != nil || len(configs) == 0 {
		return nil, err
	} else {
		return &configs[0], nil
	}
}

// GetServiceConfigHistory returns the complete config history for a service
func (s *Store) GetServiceConfigHistory(ctx datastore.Context, tenantID string, servicePath string) ([]ServiceConfigHistory, error) {
	if tenantID = strings.TrimSpace(tenantID); tenantID == "" {
		return nil, errors.New("empty TenantID not allowed")
	} else if servicePath = strings.TrimSpace(servicePath); servicePath == "" {
		return nil, errors.New("empty ServicePath not allowed")
	}

	search := search.Search("controlplane").Type(kind).Size("50000").Sort(
		search.Sort("CreatedAt").Desc(),
	).Filter(
		"and",
		search.Filter().Terms("TenantID", tenantID),
		search.Filter().Terms("ServicePath", servicePath),
	)

	q := datastore.NewQuery(ctx)
	results, err := q.Execute(search)
	if err != nil {
		return nil, err
	}

	if configs, err := convert(results); err != nil {
		return nil, err
	} else {
		return configs, nil
	}
}

// Key creates a Key suitable for getting, putting, and deleting history records
func Key(id string) datastore.Key {
	return datastore.NewKey(kind, id)
}

func convert(results datastore.Results) ([]ServiceConfigHistory, error) {
	configs := make([]ServiceConfigHistory, results.Len())
	for idx := range configs {
		var config ServiceConfigHistory
		if err := results.Get(idx, &config); err != nil {
			return nil, err
		}
		configs[idx] = config
	}
	return configs, nil
}

var (
	kind = "serviceconfighistory"
)
