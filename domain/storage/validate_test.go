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

import "testing"

func TestStorageValidate(t *testing.T) {
	storage := Storage{ID: "test_storage_id", HostID: "test_host_id", DataPath: "/path/to/data"}
	if err := storage.ValidEntity(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	storage2 := Storage{HostID: storage.HostID, DataPath: storage.DataPath}
	if err := storage2.ValidEntity(); err == nil {
		t.Errorf("Expected error")
	}

	storage3 := Storage{ID: storage.ID, DataPath: storage.DataPath}
	if err := storage3.ValidEntity(); err == nil {
		t.Errorf("Expected error")
	}

	storage4 := Storage{ID: storage.ID, HostID: storage.HostID}
	if err := storage4.ValidEntity(); err == nil {
		t.Errorf("Expected error")
	}
}
