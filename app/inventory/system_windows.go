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
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"go.qbee.io/agent/app"
	"go.qbee.io/agent/app/utils/cache"
	"golang.org/x/sys/windows"
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

	hostName, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	bootTime, err := getBootTime()
	if err != nil {
		return nil, err
	}

	platform, family, version, displayVersion, err := getPlatformInformation()
	if err != nil {
		return nil, err
	}

	kernelArch, err := KernelArch()
	if err != nil {
		return nil, err
	}

	systemInfo.BootTime = fmt.Sprintf("%d", bootTime)
	systemInfo.Host = hostName
	systemInfo.FQHost = hostName
	systemInfo.UQHost = hostName
	systemInfo.OSVersion = fmt.Sprintf("%s %s %s",
		platform,
		family,
		displayVersion,
	)
	systemInfo.Architecture = kernelArch
	systemInfo.Release = version
	systemInfo.CPUs = fmt.Sprintf("%d", runtime.NumCPU())

	systemInventory := &System{System: *systemInfo}

	cache.Set(systemInventoryCacheKey, systemInventory, systemInventoryCacheTTL)

	return systemInventory, nil
}

var (
	Modkernel32 = windows.NewLazySystemDLL("kernel32.dll")
)

var (
	procGetTickCount32 = Modkernel32.NewProc("GetTickCount")
	procGetTickCount64 = Modkernel32.NewProc("GetTickCount64")
)

func getBootTime() (uint64, error) {

	procGetTickCount := procGetTickCount64
	err := procGetTickCount64.Find()
	if err != nil {
		procGetTickCount = procGetTickCount32 // handle WinXP, but keep in mind that "the time will wrap around to zero if the system is run continuously for 49.7 days." from MSDN
	}

	up, _, lastErr := syscall.SyscallN(procGetTickCount.Addr(), 0, 0, 0, 0)
	if lastErr != 0 {
		return 0, lastErr
	}

	timeSinceMillis := uint64(time.Now().UnixMilli()) - uint64(up)

	return uint64((time.Duration(timeSinceMillis) * time.Millisecond).Seconds()), nil
}

var (
	ModNt = windows.NewLazySystemDLL("ntdll.dll")
)

var (
	procRtlGetVersion = ModNt.NewProc("RtlGetVersion")
)

// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/content/wdm/ns-wdm-_osversioninfoexw
type osVersionInfoExW struct {
	dwOSVersionInfoSize uint32
	dwMajorVersion      uint32
	dwMinorVersion      uint32
	dwBuildNumber       uint32
	dwPlatformId        uint32
	szCSDVersion        [128]uint16
	wServicePackMajor   uint16
	wServicePackMinor   uint16
	wSuiteMask          uint16
	wProductType        uint8
	wReserved           uint8
}

func getPlatformInformation() (platform, family, version, displayVersion string, err error) {
	// GetVersionEx lies on Windows 8.1 and returns as Windows 8 if we don't declare compatibility in manifest
	// RtlGetVersion bypasses this lying layer and returns the true Windows version
	// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/content/wdm/nf-wdm-rtlgetversion
	// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/content/wdm/ns-wdm-_osversioninfoexw
	var osInfo osVersionInfoExW
	osInfo.dwOSVersionInfoSize = uint32(unsafe.Sizeof(osInfo))
	ret, _, err := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&osInfo)))
	if ret != 0 {
		return //nolint:nakedret //FIXME
	}

	// Platform
	var h windows.Handle // like HostIDWithContext(), we query the registry using the raw windows.RegOpenKeyEx/RegQueryValueEx
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, windows.StringToUTF16Ptr(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`), 0, windows.KEY_READ|windows.KEY_WOW64_64KEY, &h)
	if err != nil {
		return //nolint:nakedret //FIXME
	}
	defer windows.RegCloseKey(h)
	var bufLen uint32
	var valType uint32
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`ProductName`), nil, &valType, nil, &bufLen)
	if err != nil {
		return //nolint:nakedret //FIXME
	}
	regBuf := make([]uint16, bufLen/2+1)
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`ProductName`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
	if err != nil {
		return //nolint:nakedret //FIXME
	}
	platform = windows.UTF16ToString(regBuf)
	if strings.Contains(platform, "Windows 10") { // check build number to determine whether it's actually Windows 11
		err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CurrentBuildNumber`), nil, &valType, nil, &bufLen)
		if err == nil {
			regBuf = make([]uint16, bufLen/2+1)
			err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CurrentBuildNumber`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
			if err == nil {
				buildNumberStr := windows.UTF16ToString(regBuf)
				if buildNumber, err := strconv.ParseInt(buildNumberStr, 10, 32); err == nil && buildNumber >= 22000 {
					platform = strings.Replace(platform, "Windows 10", "Windows 11", 1)
				}
			}
		}
	}
	if !strings.HasPrefix(platform, "Microsoft") {
		platform = "Microsoft " + platform
	}
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CSDVersion`), nil, &valType, nil, &bufLen) // append Service Pack number, only on success
	if err == nil {                                                                                       // don't return an error if only the Service Pack retrieval fails
		regBuf = make([]uint16, bufLen/2+1)
		err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CSDVersion`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
		if err == nil {
			platform += " " + windows.UTF16ToString(regBuf)
		}
	}

	var UBR uint32 // Update Build Revision
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`UBR`), nil, &valType, nil, &bufLen)
	if err == nil {
		regBuf := make([]byte, 4)
		err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`UBR`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
		copy((*[4]byte)(unsafe.Pointer(&UBR))[:], regBuf)
	}

	// Get DisplayVersion(ex: 23H2) as platformVersion
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`DisplayVersion`), nil, &valType, nil, &bufLen)
	if err == nil {
		regBuf := make([]uint16, bufLen/2+1)
		err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`DisplayVersion`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
		displayVersion = windows.UTF16ToString(regBuf)
	}

	// PlatformFamily
	switch osInfo.wProductType {
	case 1:
		family = "Standalone Workstation"
	case 2:
		family = "Server (Domain Controller)"
	case 3:
		family = "Server"
	}

	// Platform Version
	version = fmt.Sprintf("%d.%d.%d.%d Build %d.%d",
		osInfo.dwMajorVersion, osInfo.dwMinorVersion, osInfo.dwBuildNumber, UBR,
		osInfo.dwBuildNumber, UBR)

	return platform, family, version, displayVersion, nil
}

var (
	procGetNativeSystemInfo = Modkernel32.NewProc("GetNativeSystemInfo")
)

type systemInfo struct {
	wProcessorArchitecture      uint16
	wReserved                   uint16
	dwPageSize                  uint32
	lpMinimumApplicationAddress uintptr
	lpMaximumApplicationAddress uintptr
	dwActiveProcessorMask       uintptr
	dwNumberOfProcessors        uint32
	dwProcessorType             uint32
	dwAllocationGranularity     uint32
	wProcessorLevel             uint16
	wProcessorRevision          uint16
}

func KernelArch() (string, error) {
	var systemInfo systemInfo
	procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&systemInfo)))

	const (
		PROCESSOR_ARCHITECTURE_INTEL = 0
		PROCESSOR_ARCHITECTURE_ARM   = 5
		PROCESSOR_ARCHITECTURE_ARM64 = 12
		PROCESSOR_ARCHITECTURE_IA64  = 6
		PROCESSOR_ARCHITECTURE_AMD64 = 9
	)
	switch systemInfo.wProcessorArchitecture {
	case PROCESSOR_ARCHITECTURE_INTEL:
		if systemInfo.wProcessorLevel < 3 {
			return "i386", nil
		}
		if systemInfo.wProcessorLevel > 6 {
			return "i686", nil
		}
		return fmt.Sprintf("i%d86", systemInfo.wProcessorLevel), nil
	case PROCESSOR_ARCHITECTURE_ARM:
		return "arm", nil
	case PROCESSOR_ARCHITECTURE_ARM64:
		return "aarch64", nil
	case PROCESSOR_ARCHITECTURE_IA64:
		return "ia64", nil
	case PROCESSOR_ARCHITECTURE_AMD64:
		return "x86_64", nil
	}
	return "", nil
}
