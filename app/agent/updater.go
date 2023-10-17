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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/qbee-io/qbee-agent/app/binary"
)

// Update the agent binary.
func Update(ctx context.Context, cfg *Config) error {
	agent, err := New(cfg)
	if err != nil {
		return fmt.Errorf("cannot initialize agent: %w", err)
	}

	return agent.updateAgent(ctx)
}

func (agent *Agent) updateAgent(ctx context.Context) error {
	// let's not block for more than the run interval
	ctxWithTimeout, cancel := context.WithTimeout(ctx, agent.Configuration.RunInterval())
	defer cancel()

	// determine the agent binary path
	agentBinPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		return fmt.Errorf("cannot determine agent path: %w", err)
	}

	if agentBinPath, err = filepath.Abs(agentBinPath); err != nil {
		return fmt.Errorf("cannot determine absolute agent path: %w", err)
	}

	if err = binary.Download(agent.api, ctxWithTimeout, binary.Agent, agentBinPath); err != nil {
		return fmt.Errorf("cannot download agent binary: %w", err)
	}

	// stop the agent
	agent.stop <- true

	return nil
}
