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

package cmd

import (
	"fmt"

	"qbee.io/platform/utils/cmd"

	"github.com/qbee-io/qbee-agent/app"
)

var versionCommand = cmd.Command{
	Description: "Agent version.",
	Target: func(opts cmd.Options) error {
		fmt.Printf("%s (commit: %s)\n", app.Version, app.Commit)
		return nil
	},
}
