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
	"testing"

	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/datastore/elastic"
	"github.com/control-center/serviced/domain/servicedefinition"
	. "gopkg.in/check.v1"
)

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
	store *Store
}

func (s *S) SetUpTest(c *C) {
	s.ElasticTest.SetUpTest(c)
	datastore.Register(s.Driver())
	s.ctx = datastore.Get()
	s.store = NewStore()
}

func (s *S) TestServiceConfigHistory_CRUD(t *C) {
	expected, err := NewRecord("svc_test_id", "./a/b/c", "initial revision", make(map[string]servicedefinition.ConfigFile, 0))
	t.Assert(err, IsNil)

	actual, err := s.store.Get(s.ctx, expected.ID)
	t.Assert(err, NotNil)
	if !datastore.IsErrNoSuchEntity(err) {
		t.Fatalf("unexpected error type: %v", err)
	}

	err = s.store.Put(s.ctx, expected)
	t.Assert(err, IsNil)

	// Test update
	expected.CommitMessage = "new test message"
	err = s.store.Put(s.ctx, expected)
	t.Assert(err, IsNil)

	actual, err = s.store.Get(s.ctx, expected.ID)
	t.Assert(err, IsNil)

	actual.SetDatabaseVersion(0)
	t.Assert(expected, DeepEquals, actual)

	// Test delete
	err = s.store.Delete(s.ctx, expected.ID)
	t.Assert(err, IsNil)

	actual, err = s.store.Get(s.ctx, expected.ID)
	t.Assert(err, NotNil)
	if !datastore.IsErrNoSuchEntity(err) {
		t.Fatalf("unexpected error type: %v", err)
	}
}

func (s *S) TestServiceConfigHistory_Commit(t *C) {
	equals := func(actual, expected ServiceConfigHistory) {
		t.Assert(actual.ID, Equals, expected.ID)
		t.Assert(actual.TenantID, Equals, expected.TenantID)
		t.Assert(actual.ServicePath, Equals, expected.ServicePath)
		t.Assert(actual.CreatedAt.Unix() > 0, Equals, true)
		t.Assert(actual.CommitMessage, Equals, expected.CommitMessage)
		t.Assert(actual.ConfigFiles, HasLen, len(expected.ConfigFiles))
		for name, file := range expected.ConfigFiles {
			t.Assert(actual.ConfigFiles[name], DeepEquals, file)
		}
	}

	verify := func(config *ServiceConfigHistory, configs ...ServiceConfigHistory) {
		var err error
		err = s.store.Put(s.ctx, config)
		t.Assert(err, IsNil)

		actual1, err := s.store.GetServiceConfig(s.ctx, config.TenantID, config.ServicePath)
		t.Assert(err, IsNil)
		equals(*actual1, *config)

		expected := append([]ServiceConfigHistory{*config}, configs...)
		actual2, err := s.store.GetServiceConfigHistory(s.ctx, config.TenantID, config.ServicePath)
		t.Assert(err, IsNil)
		t.Assert(actual2, HasLen, len(expected))
		for i := range expected {
			equals(actual2[i], expected[i])
		}
	}

	configfiles := map[string]servicedefinition.ConfigFile{
		"file1": servicedefinition.ConfigFile{Filename: "file1", Owner: "testuser", Permissions: "0777", Content: "config file1 data"},
		"file2": servicedefinition.ConfigFile{Filename: "file2", Owner: "testuser", Permissions: "0777", Content: "config file2 data"},
	}
	record, err := NewRecord("tenant_test_id1", "./a/b/c", "test message 1", configfiles)
	t.Assert(err, IsNil)

	actual1, err := s.store.GetServiceConfig(s.ctx, record.TenantID, record.ServicePath)
	t.Assert(err, IsNil)
	t.Assert(actual1, IsNil)

	actual2, err := s.store.GetServiceConfigHistory(s.ctx, record.TenantID, record.ServicePath)
	t.Assert(err, IsNil)
	t.Assert(actual2, HasLen, 0)

	verify(record)
	record, err = s.store.GetServiceConfig(s.ctx, record.TenantID, record.ServicePath)
	t.Assert(err, IsNil)

	configfiles["file3"] = servicedefinition.ConfigFile{Filename: "file3", Content: "config file3 data"}
	record1, err := NewRecord("tenant_test_id1", "./a/b/c", "test message 2", configfiles)
	t.Assert(err, IsNil)
	verify(record1, *record)

	record2, err := NewRecord("tenant_test_id1", "./1/2/3", "test message 3", configfiles)
	t.Assert(err, IsNil)
	verify(record2)

	record3, err := NewRecord("tenant_test_id2", "./a/b/c", "test message 4", configfiles)
	t.Assert(err, IsNil)
	verify(record3)

	record4, err := NewRecord("tenant_test_id2", "./1/2/3", "test message 5", configfiles)
	t.Assert(err, IsNil)
	verify(record4)
}
