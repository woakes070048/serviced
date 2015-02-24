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
	"testing"

	. "gopkg.in/check.v1"

	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/datastore/elastic"
)

// Test plumbs gocheck into testing
func Test(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&S{
	ElasticTest: elastic.ElasticTest{
		Index:    "controlplane",
		Mappings: []elastic.Mapping{MAPPING},
	}})

type S struct {
	elastic.ElasticTest
	ctx   datastore.Context
	store Store
}

func (s *S) SetUpTest(c *C) {
	s.ElasticTest.SetUpTest(c)
	datastore.Register(s.Driver())
	s.ctx = datastore.Get()
	s.store = NewStore()
}

func (s *S) Test_StorageCRUD(t *C) {
	storage := &Storage{ID: "store_test_id", HostID: "test_host_id", DataPath: "/path/to/data"}

	// Test create
	storage2, err := s.store.Get(s.ctx, storage.ID)
	t.Assert(err, NotNil)
	t.Assert(datastore.IsErrNoSuchEntity(err), Equals, true)

	err = s.store.Put(s.ctx, storage)
	t.Assert(err, IsNil)

	storage.DatabaseVersion = 1
	storage2, err = s.store.Get(s.ctx, storage.ID)
	t.Assert(err, IsNil)
	t.Assert(storage2, DeepEquals, storage)

	// Test update
	storage.OSDID = 2
	storage.JournalPath = "/path/to/journal"
	storage.Weight = 1.0

	err = s.store.Put(s.ctx, storage)
	t.Assert(err, IsNil)

	storage.DatabaseVersion = 2
	storage2, err = s.store.Get(s.ctx, storage.ID)
	t.Assert(err, IsNil)
	t.Assert(storage2, DeepEquals, storage)

	// Test delete
	err = s.store.Delete(s.ctx, storage.ID)
	t.Assert(err, IsNil)

	storage2, err = s.store.Get(s.ctx, storage.ID)
	t.Assert(err, NotNil)
	t.Assert(datastore.IsErrNoSuchEntity(err), Equals, true)
}

func (s *S) Test_GetAll(t *C) {
	storages := []Storage{
		{ID: "store_test_id-1", HostID: "test_host_id-1", DataPath: "/path/to/data1"},
		{ID: "store_test_id-2", HostID: "test_host_id-2", DataPath: "/path/to/data2"},
		{ID: "store_test_id-3", HostID: "test_host_id-2", DataPath: "/path/to/data3"},
	}

	storages2, err := s.store.GetAll(s.ctx)
	t.Assert(err, IsNil)
	t.Assert(storages2, HasLen, 0)

	storagemap := make(map[string]Storage)
	for _, storage := range storages {
		err = s.store.Put(s.ctx, &storage)
		t.Assert(err, IsNil)

		storage.DatabaseVersion = 1
		storagemap[storage.ID] = storage
	}

	storages2, err = s.store.GetAll(s.ctx)
	t.Assert(err, IsNil)
	t.Assert(storages2, HasLen, len(storages))

	for _, storage2 := range storages2 {
		storage, ok := storagemap[storage2.ID]
		t.Assert(ok, Equals, true)
		t.Assert(storage2, DeepEquals, storage)
	}
}

func (s *S) Test_GetByHost(t *C) {
	storages := []Storage{
		{ID: "store_test_id-1", HostID: "test_host_id-1", DataPath: "/path/to/data1"},
		{ID: "store_test_id-2", HostID: "test_host_id-2", DataPath: "/path/to/data2"},
		{ID: "store_test_id-3", HostID: "test_host_id-2", DataPath: "/path/to/data3"},
	}

	storagemap := make(map[string]Storage)
	for _, storage := range storages {
		err := s.store.Put(s.ctx, &storage)
		t.Assert(err, IsNil)

		storage.DatabaseVersion = 1
		storagemap[storage.ID] = storage
	}

	verify := func(hostID string, size int) {
		storages2, err := s.store.GetByHost(s.ctx, hostID)
		t.Assert(err, IsNil)
		t.Assert(storages2, HasLen, size)

		for _, storage2 := range storages2 {
			t.Assert(storage2.HostID, Equals, hostID)
			storage, ok := storagemap[storage2.ID]
			t.Assert(ok, Equals, true)
			t.Assert(storage2, DeepEquals, storage)
		}
	}

	verify("test_host_id-0", 0)
	verify("test_host_id-1", 1)
	verify("test_host_id-2", 2)
}
