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
	"fmt"

	"github.com/control-center/serviced/datastore"
	"github.com/zenoss/elastigo/search"
)

//NewStore creates a Service  store
func NewStore() *Store {
	return &Store{}
}

//Store type for interacting with Service persistent storage
type Store struct {
	ds datastore.DataStore
}

// Put adds or updates a ServiceConfig
func (s *Store) Put(ctx datastore.Context, conf *ServiceConfig) error {
	return s.ds.Put(ctx, Key(conf.ID), conf)
}

// Get gets a particular ServiceConfig
func (s *Store) Get(ctx datastore.Context, id string) (*ServiceConfig, error) {
	var conf ServiceConfig
	if err := s.ds.Get(ctx, Key(id), conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

// GetLatestServiceConfig returns the latest config file for a service
func (s *Store) GetLatestServiceConfig(ctx datastore.Context, serviceID string) (*ServiceConfig, error) {
	querystring := fmt.Sprintf("ServiceID:%s", serviceID)
	search := search.Search("controlplane").Size("1").Query(querystring).Sort("Updated").Desc()

	q := datastore.NewQuery(ctx)
	results, err := q.Execute(search)
	if err != nil {
		return nil, err
	}

	if results.Len() == 0 {
		return nil, nil
	}

	var conf ServiceConfig
	if err := results.Get(0, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

// GetConfigHistory gets the entire config history for a particular service
func (s *Store) GetServiceConfigHistory(ctx datastore.Context, serviceID string) ([]ServiceConfig, error) {
	querystring := fmt.Sprintf("ServiceID:%s", serviceID)
	search := search.Search("controlplane").Size("50000").Query(querystring).Sort("Updated").Desc()

	q := datastore.NewQuery(ctx)
	results, err := q.Execute(search)
	if err != nil {
		return nil, err
	}

	return convert(results)
}

func convert(results datastore.Results) ([]ServiceConfig, error) {
	result := make([]ServiceConfig, results.Len())
	for idx := range result {
		var cf SvcConfigFile
		err := results.Get(idx, &cf)
		if err != nil {
			return nil, err
		}
		result[idx] = cf
	}
	return result, nil
}

//Key creates a Key suitable for getting, putting and deleting SvcConfigFile
func Key(id string) datastore.Key {
	return datastore.NewKey(kind, id)
}

var (
	kind = "svcconfigfile"
)
