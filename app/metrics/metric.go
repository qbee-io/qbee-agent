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

package metrics

// Label defines a resource type for which metrics are collected.
type Label string

// Collected metric labels.
const (
	CPU         Label = "cpu"
	Memory      Label = "memory"
	Filesystem  Label = "filesystem"
	LoadAverage Label = "loadavg_weighted"
	Network     Label = "network"
	Temperature Label = "temperature"
)

// Metric defines the base metric data structure.
type Metric struct {
	// Label identifies the type of the metric.
	Label Label `json:"label"`

	// Timestamp defines when the metric was recorded.
	Timestamp int64 `json:"ts"`

	// ID is an optional metric identifier.
	ID string `json:"id,omitempty"`

	// Values contain metric values.
	Values Values `json:"values"`
}

// Values combines values from all labels in one struct.
// Values without data, won't be stored in database nor marshaled into JSON.
type Values struct {
	*CPUValues         `json:",omitempty"`
	*MemoryValues      `json:",omitempty"`
	*FilesystemValues  `json:",omitempty"`
	*LoadAverageValues `json:",omitempty"`
	*NetworkValues     `json:",omitempty"`
	*TemperatureValues `json:",omitempty"`
}
