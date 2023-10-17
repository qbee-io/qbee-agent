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

package agent

import (
	"testing"

	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/runner"
)

func Test_Update_Manual(t *testing.T) {
	r := runner.New(t)
	r.Bootstrap()

	version := r.MustExec("qbee-agent", "version")

	assert.Equal(t, string(version), "0000.00 (commit: NA)")

	r.MustExec("qbee-agent", "-l", "DEBUG", "update")

	version = r.MustExec("qbee-agent", "version")

	assert.Equal(t, string(version), "2023.01 (commit: NA)")
}

func Test_Update_Manual_UsingRelativePath(t *testing.T) {
	r := runner.New(t)
	r.Bootstrap()

	assert.Equal(t, string(r.MustExec("pwd")), "/app")
	r.MustExec("cp", "/usr/sbin/qbee-agent", "/app/qbee-agent")
	r.MustExec("./qbee-agent", "-l", "DEBUG", "update")

	// make sure the original binary is not overwritten
	version := r.MustExec("qbee-agent", "version")
	assert.Equal(t, string(version), "0000.00 (commit: NA)")

	// make sure the binary we used to update is updated
	version = r.MustExec("/app/qbee-agent", "version")
	assert.Equal(t, string(version), "2023.01 (commit: NA)")
}

func Test_Update_Automatic(t *testing.T) {
	r := runner.New(t)

	// since auto-update is disabled by default,
	// we need to enable it during bootstrap with --enable-auto-update flag
	r.Bootstrap("--enable-auto-update")

	version := r.MustExec("qbee-agent", "version")

	assert.Equal(t, string(version), "0000.00 (commit: NA)")

	// Agent always starts with initial run and that will trigger the update.
	// When th update is successful, the agent will terminate itself.
	r.MustExec("qbee-agent", "start")

	version = r.MustExec("qbee-agent", "version")

	assert.Equal(t, string(version), "2023.01 (commit: NA)")
}
