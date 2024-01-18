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

//go:build linux

package inventory

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go.qbee.io/agent/app"
	"go.qbee.io/agent/app/inventory/linux"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils"
)

const (
	systemClass = "linux"
	systemOS    = "linux"
)

// CollectSystemInventory returns populated System inventory based on current system status.
func CollectSystemInventory() (*System, error) {
	systemInfo := &SystemInfo{
		AgentVersion: app.Version,
		Class:        systemClass,
		OS:           systemOS,
		VPNIndex:     "1",
	}

	if err := systemInfo.parseOSRelease(); err != nil {
		return nil, err
	}

	if err := systemInfo.parseCPUInfo(); err != nil {
		return nil, err
	}

	if err := systemInfo.parseUnameSyscall(); err != nil {
		return nil, err
	}

	if err := systemInfo.parseSysinfoSyscall(); err != nil {
		return nil, err
	}

	if err := systemInfo.gatherNetworkInfo(); err != nil {
		return nil, err
	}

	systemInfo.OSType = fmt.Sprintf("%s_%s", systemInfo.OS, systemInfo.Architecture)
	systemInfo.LongArchitecture = canonify(fmt.Sprintf("%s_%s_%s_%s",
		systemInfo.OS,
		systemInfo.Architecture,
		systemInfo.Release,
		systemInfo.Version))

	systemInfo.CPUs = fmt.Sprintf("%d", runtime.NumCPU())

	systemInventory := &System{System: *systemInfo}

	return systemInventory, nil
}

// getDefaultNetworkInterface returns a default network interface name.
func (systemInfo *SystemInfo) getDefaultNetworkInterface() (string, error) {
	routeFilePath := filepath.Join(linux.ProcFS, "net", "route")

	defaultInterface := ""

	err := utils.ForLinesInFile(routeFilePath, func(line string) error {
		fields := strings.Fields(line)
		if fields[1] == "Destination" {
			return nil
		}

		if defaultInterface == "" {
			defaultInterface = fields[0]
		}

		if fields[1] == "00000000" && defaultInterface == "" {
			defaultInterface = fields[0]
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("error getting default network interface: %w", err)
	}

	return defaultInterface, nil
}

// gatherNetworkInfo gathers system's networking configuration.
func (systemInfo *SystemInfo) gatherNetworkInfo() error {
	defaultNetworkInterface, err := systemInfo.getDefaultNetworkInterface()
	if err != nil {
		return err
	}

	systemInfo.Interface = defaultNetworkInterface

	var interfaces []net.Interface
	if interfaces, err = net.Interfaces(); err != nil {
		return fmt.Errorf("error gaterhing network interfaces info: %w", err)
	}

	systemInfo.HardwareMAC = make(map[string]string)
	systemInfo.InterfaceFlags = make(map[string]string)
	systemInfo.IPv4 = make(map[string]string)
	systemInfo.IPv6 = make(map[string]string)
	addresses := make([]string, 0, len(interfaces))

	for _, iface := range interfaces {
		// skip all loopback interfaces
		if iface.Flags&net.FlagLoopback > 0 {
			continue
		}

		systemInfo.HardwareMAC[iface.Name] = iface.HardwareAddr.String()
		systemInfo.InterfaceFlags[iface.Name] = strings.ReplaceAll(iface.Flags.String(), "|", " ")

		var ifaceAddresses []net.Addr
		if ifaceAddresses, err = iface.Addrs(); err != nil {
			return fmt.Errorf("error gathering IP addresses for interface %s: %w", iface.Name, err)
		}

		ipv4 := make([]string, 0, len(ifaceAddresses))
		ipv6 := make([]string, 0, len(ifaceAddresses))

		for _, addr := range ifaceAddresses {
			ipAddress := addr.(*net.IPNet)

			isIPv4 := len(ipAddress.IP.To4()) == net.IPv4len
			if isIPv4 {
				ipv4 = append(ipv4, ipAddress.IP.String())
				addresses = append(addresses, ipAddress.IP.String())
			} else {
				ipv6 = append(ipv6, ipAddress.IP.String())
			}
		}

		if iface.Name == defaultNetworkInterface && len(ipv4) > 0 {
			systemInfo.IPv4First = ipv4[0]
		}

		if len(ipv4) > 0 {
			systemInfo.IPv4[iface.Name] = strings.Join(ipv4, " ")
		}

		if len(ipv6) > 0 {
			systemInfo.IPv6[iface.Name] = strings.Join(ipv6, " ")
		}
	}

	systemInfo.IPAddresses = strings.Join(addresses, " ")

	return nil
}

// parseOSRelease extracts flavor information from os-release file.
func (systemInfo *SystemInfo) parseOSRelease() error {
	// Set default to unknown
	systemInfo.Flavor = "unknown"
	data, err := utils.ParseEnvFile("/etc/os-release")
	if err != nil {
		data, err = utils.ParseEnvFile("/usr/lib/os-release")
	}

	if err != nil {
		log.Debugf("error parsing os-release file: %v", err)
		return nil
	}

	id := canonify(strings.ToLower(data["ID"]))
	versionID := canonify(data["VERSION_ID"])

	version := strings.Split(versionID, "_")

	systemInfo.Flavor = fmt.Sprintf("%s_%s", id, version[0])
	return nil
}

// parseCPUInfo parses /proc/cpuinfo for extra details re. CPU.
func (systemInfo *SystemInfo) parseCPUInfo() error {
	filePath := filepath.Join(linux.ProcFS, "cpuinfo")

	const expectedLineSubstrings = 2

	return utils.ForLinesInFile(filePath, func(line string) error {
		line = strings.TrimSpace(line)

		substrings := strings.SplitN(line, ":", expectedLineSubstrings)
		if len(substrings) != expectedLineSubstrings {
			return nil
		}

		key := strings.TrimSpace(substrings[0])

		switch key {
		case "Serial":
			systemInfo.CPUSerialNumber = strings.TrimSpace(substrings[1])
		case "Hardware":
			systemInfo.CPUHardware = strings.TrimSpace(substrings[1])
		case "Revision":
			systemInfo.CPURevision = strings.TrimSpace(substrings[1])
		}

		return nil
	})
}

// parseSysinfoSyscall populates system info from sysinfo system call.
func (systemInfo *SystemInfo) parseSysinfoSyscall() error {
	now := time.Now()
	sysinfo, err := getSysinfo()
	if err != nil {
		return err
	}

	systemInfo.BootTime = fmt.Sprintf("%d", now.Unix()-int64(sysinfo.Uptime))

	return nil
}

// getSysinfo returns sysinfo struct.
func getSysinfo() (*syscall.Sysinfo_t, error) {
	sysinfo := new(syscall.Sysinfo_t)
	if err := syscall.Sysinfo(sysinfo); err != nil {
		return nil, fmt.Errorf("error calling sysinfo syscall: %w", err)
	}

	return sysinfo, nil
}

var nonAlphaNumRE = regexp.MustCompile("[^a-zA-Z0-9]")

// canonify replaces all non-alphanumeric characters with underscore (_).
func canonify(val string) string {
	return nonAlphaNumRE.ReplaceAllString(val, "_")
}
