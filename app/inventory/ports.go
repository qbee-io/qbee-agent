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

package inventory

// TypePorts is the inventory type for listening network ports.
const TypePorts Type = "ports"

// Ports contains information about currently listening network ports.
type Ports struct {
	Ports []Port `json:"items"`
}

// Port contains information about a listening network port.
type Port struct {
	// Protocol - network protocol used (e.g. "tcp", "tcp6", "udp" or "udp6").
	Protocol string `json:"proto"`

	// Socket - which socket is listening (e.g. "0.0.0.0:69").
	Socket string `json:"socket"`

	// Process - which process is controlling the socket (e.g. "/usr/sbin/in.tftpd ...").
	Process string `json:"proc_info"`
}
