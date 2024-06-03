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
	"context"
	"reflect"
	"testing"
)

/*

cgi-io - 2021-09-08-98cef9dd-20 - 2022-08-10-901b0f04-21
opkg - 2021-06-13-1bf042dd-1 - 2021-06-13-1bf042dd-2
luci-app-opkg - git-21.312.69848-4745991 - git-22.273.29015-e01e38c
libiwinfo-lua - 2021-04-30-c45f0b58-2.1 - 2022-08-19-0dad3e66-1
luci-mod-system - git-22.019.40321-7a37d02 - git-23.013.73129-aa7938d
libustream-wolfssl20201210 - 2022-01-16-868fd881-1 - 2022-01-16-868fd881-2
luci-theme-bootstrap - git-22.084.39047-f1d687e - git-22.288.45155-afd0012
dnsmasq - 2.85-8 - 2.85-9
procd - 2021-03-08-2cfc26f8-1 - 2021-03-08-2cfc26f8-2
px5g-wolfssl - 3 - 4.1
luci-mod-status - git-22.046.85784-0ac2542 - git-22.089.70019-d4f0b06
firewall - 2021-03-23-61db17ed-1 - 2021-03-23-61db17ed-1.1
uclient-fetch - 2021-05-14-6a6011df-1 - 2023-04-13-007d9454-1
libiwinfo-data - 2021-04-30-c45f0b58-2.1 - 2022-08-19-0dad3e66-1
luci-base - git-22.083.69138-0a0ce2a - git-23.093.57360-e98243e
libiwinfo20210430 - 2021-04-30-c45f0b58-2.1 - 2022-08-19-0dad3e66-1
ca-bundle - 20210119-1 - 20211016-1
libuclient20201210 - 2021-05-14-6a6011df-1 - 2023-04-13-007d9454-1
luci-mod-network - git-22.046.85061-dd54dce - git-22.244.54918-77c916e
*/

func TestOpkgPackageManager_parseUpdateAvailableLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want *Package
	}{
		{
			name: "test",
			line: "cgi-io - 2021-09-08-98cef9dd-20 - 2022-08-10-901b0f04-21",
			want: &Package{
				Name:         "cgi-io",
				Version:      "2021-09-08-98cef9dd-20",
				Architecture: "",
				Update:       "2022-08-10-901b0f04-21",
			},
		},
		{
			name: "invalid line",
			line: "this line is not valid",
			want: nil,
		},
	}
	opkg := &OpkgPackageManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := opkg.parseUpdateAvailableLine(tt.line); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseUpdateAvailableLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpkgPackageManager_parsePackageFile(t *testing.T) {

	tests := []struct {
		name string
		file string
		want *Package
	}{
		{
			name: "test-simple-architecture",
			file: "/path/to/package_1.1.1-1_ar71xx.ipk",
			want: &Package{
				Name:         "package",
				Version:      "1.1.1-1",
				Architecture: "ar71xx",
			},
		},
		{
			name: "test-underscore-in-architecture",
			file: "/path/to/package-with-long-name_1.1.1-1_x86_64.ipk",
			want: &Package{
				Name:         "package-with-long-name",
				Version:      "1.1.1-1",
				Architecture: "x86_64",
			},
		},
		{
			name: "test-invalid-file",
			file: "/path/to/invalid_package_1.1.1-1_ar71xx.ipk",
			want: nil,
		},
	}
	ctx := context.Background()
	opkg := &OpkgPackageManager{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := opkg.ParsePackageFile(ctx, tt.file); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePackageFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
