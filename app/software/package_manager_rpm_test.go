// Copyright 2024 qbee.io
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

package software

import (
	"strings"
	"testing"
)

func TestRpmPackageArchitecture(t *testing.T) {

}

func TestRpmUpdatesParse(t *testing.T) {

	updateLines := `
curl-minimal.x86_64                 7.76.1-26.el9_3.3       ubi-9-baseos-rpms   
gdb-gdbserver.x86_64                10.2-11.1.el9_3         ubi-9-appstream-rpms
glibc.x86_64                        2.34-83.el9_3.12        ubi-9-baseos-rpms   
glibc-common.x86_64                 2.34-83.el9_3.12        ubi-9-baseos-rpms   
glibc-minimal-langpack.x86_64       2.34-83.el9_3.12        ubi-9-baseos-rpms   
gnutls.x86_64                       3.7.6-23.el9_3.3        ubi-9-baseos-rpms   
libcurl-minimal.x86_64              7.76.1-26.el9_3.3       ubi-9-baseos-rpms   
openssl.x86_64                      1:3.0.7-25.el9_3        ubi-9-baseos-rpms   
openssl-libs.x86_64                 1:3.0.7-25.el9_3        ubi-9-baseos-rpms   
python3-pip-wheel.noarch            21.2.3-7.el9_3.1        ubi-9-baseos-rpms   
tzdata.noarch                       2024a-1.el9             ubi-9-baseos-rpms   
`
	pkgMgr := &RpmPackageManager{}

	lines := strings.Split(updateLines, "\n")

	for _, line := range lines {

		pkg := pkgMgr.parseUpdateAvailableLine(line)
		if pkg == nil {
			t.Errorf("Failed to parse line: %s", line)
		} else {
			t.Errorf("Parsed package: %+v", pkg)
		}
	}

}
