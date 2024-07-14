// Copyright 2023 qbee.io
//
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
//
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package metrics

// FilesystemValues represents filesystem metric values.
//
// Example payload:
//
//	{
//	 "avail": 399848008,
//	 "use": 14,
//	}
type FilesystemValues struct {
	Available uint64 `json:"avail"`
	Use       uint64 `json:"use"`
}

const fsBlockSize = 1024

// CollectFilesystem returns filesystem metric for each filesystem mounted in read-write mode.
func CollectFilesystem() ([]Metric, error) {
	return nil, nil
}
