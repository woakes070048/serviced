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

import "github.com/control-center/serviced/validation"

// ValidEntity validates storage fields
func (s *Storage) ValidEntity() error {
	violations := validation.NewValidationError()
	violations.Add(validation.NotEmpty("ID", s.ID))
	violations.Add(validation.NotEmpty("HostID", s.HostID))
	violations.Add(validation.NotEmpty("DataPath", s.DataPath))

	if violations.HasError() {
		return violations
	}
	return nil
}
