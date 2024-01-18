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

package main

import (
	"fmt"
	"os"
	"syscall"

	"go.qbee.io/agent/app/cmd"
)

var defaultUmask int = 0077

func init() {
	// set global umask
	syscall.Umask(defaultUmask)
}

func main() {
	if err := cmd.Main.Execute(os.Args[1:], nil); err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}
