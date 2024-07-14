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

package inventory

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/host"
	"go.qbee.io/agent/app"
	"go.qbee.io/agent/app/utils/cache"
)

const (
	systemClass = "windows"
	systemOS    = "windows"
)

func CollectSystemInventory(tpmEnabled bool) (*System, error) {
	if cachedItem, ok := cache.Get(systemInventoryCacheKey); ok {
		return cachedItem.(*System), nil
	}

	systemInfo := &SystemInfo{
		AgentVersion: app.Version,
		TPMEnabled:   tpmEnabled,
		Class:        systemClass,
		OS:           systemOS,
		VPNIndex:     "1",
	}
	infoStat, _ := host.Info()

	systemInfo.BootTime = fmt.Sprintf("%d", infoStat.BootTime)
	systemInfo.Host = infoStat.Hostname
	systemInfo.FQHost = infoStat.Hostname
	systemInfo.UQHost = infoStat.Hostname
	systemInfo.OSVersion = fmt.Sprintf("%s %s %s",
		infoStat.Platform,
		infoStat.PlatformFamily,
		infoStat.PlatformVersion,
	)
	systemInfo.Architecture = infoStat.KernelArch
	systemInfo.Release = infoStat.KernelVersion
	systemInfo.CPUs = fmt.Sprintf("%d", infoStat.Procs)

	return &System{
		System: *systemInfo,
	}, nil
}
