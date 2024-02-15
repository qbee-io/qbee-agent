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

package utils

import (
	"fmt"
	"os/exec"
	"runtime"
)

// GuesssUpstartCommand guesses the upstart system based on available binaries
func GuessUpstartCommand(progName, command string) ([]string, error) {
	// up%s is only used on linux
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("Unsupported OS: %s", runtime.GOOS)
	}
	// first check for systemd
	if _, err := exec.LookPath("systemctl"); err == nil {
		return []string{"systemctl", command, progName}, nil
	}
	// then check for sysvinit
	if _, err := exec.LookPath("service"); err == nil {
		return []string{"service", progName, command}, nil
	}
	// then check for openrc
	if _, err := exec.LookPath("rc-service"); err == nil {
		return []string{"rc-service", progName, command}, nil
	}
	// then check for upstart
	if _, err := exec.LookPath("initctl"); err == nil {
		return []string{"initctl", command, progName}, nil
	}
	// then check for runit
	if _, err := exec.LookPath("sv"); err == nil {
		return []string{"sv", command, progName}, nil
	}
	// then check for launchctl
	if _, err := exec.LookPath("launchctl"); err == nil {
		return []string{"launchctl", command, progName}, nil
	}
	// then check for rcctl
	if _, err := exec.LookPath("rcctl"); err == nil {
		return []string{"rcctl", command, progName}, nil
	}
	// then check existence of /etc/init.d/qbee-agent
	if _, err := exec.LookPath(fmt.Sprintf("/etc/init.d/%s", progName)); err == nil {
		return []string{fmt.Sprintf("/etc/init.d/%s", progName), command}, nil
	}

	return nil, fmt.Errorf("No supported init system found")
}
