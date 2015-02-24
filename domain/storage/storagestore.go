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
	"strings"

	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain"
	"github.com/zenoss/elastigo/search"
)

type Store interface {
	Put(ctx datastore.Context, s *Storage) error
	Get(ctx datastore.Context, id string) (*Storage, error)
	Delete(ctx datastore.Context, id string) error
	GetAll(ctx datastore.Context) ([]Storage, error)
	GetByHost(ctx datastore.Context, hostID string) ([]Storage, error)
}

type StorageStore struct {
	ds datastore.DataStore
}

func NewStore() Store {
	return &StorageStore{}
}

// Put adds or updates a storage item
func (s *StorageStore) Put(ctx datastore.Context, storage *Storage) error {
	return s.ds.Put(ctx, Key(storage.ID), storage)
}

// Get gets a storage item by id.  Return ErrNoSuchEntity if not found
func (s *StorageStore) Get(ctx datastore.Context, id string) (*Storage, error) {
	var storage Storage
	if err := s.ds.Get(ctx, Key(id), &storage); err != nil {
		return nil, err
	}
	return &storage, nil
}

// Delete removes a storage item if it exists
func (s *StorageStore) Delete(ctx datastore.Context, id string) error {
	return s.ds.Delete(ctx, Key(id))
}

// GetAll gets all storage items
func (s *StorageStore) GetAll(ctx datastore.Context) ([]Storage, error) {
	query := search.Query().Search("_exists_:ID")
	search := search.Search("controlplane").Type(kind).Size("50000").Query(query)
	results, err := datastore.NewQuery(ctx).Execute(search)
	if err != nil {
		return nil, err
	}
	return convert(results)
}

//GetAllByHostID gets all storage items for a particular host
func (s *StorageStore) GetByHost(ctx datastore.Context, hostID string) ([]Storage, error) {
	if hostID = strings.TrimSpace(hostID); hostID == "" {
		return nil, domain.EmptySearchTerm("HostID")
	}

	query := search.Query().Term("HostID", hostID)
	search := search.Search("controlplane").Type(kind).Size("50000").Query(query)
	results, err := datastore.NewQuery(ctx).Execute(search)
	if err != nil {
		return nil, err
	}
	return convert(results)
}

// Key creates a Key suitable for getting, putting and deleting storage items
func Key(id string) datastore.Key {
	return datastore.NewKey(kind, id)
}

func convert(results datastore.Results) ([]Storage, error) {
	storages := make([]Storage, results.Len())
	for idx := range storages {
		if err := results.Next(&storages[idx]); err != nil {
			return nil, err
		}
	}
	return storages, nil
}

var kind = "storage"
