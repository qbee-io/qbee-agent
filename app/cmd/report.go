// Copyright 2025 qbee.io
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
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/cmd"
)

const (
	reportSeverityOption = "severity"
	reportTextOption     = "text"
	reportBundleOption   = "bundle"
	reportLabelsOption   = "labels"
)

// validateSeverity validates the severity value
func validateSeverity(severity string) error {
	validSeverities := map[string]bool{
		"INFO": true,
		"WARN": true,
		"ERR":  true,
		"CRIT": true,
	}
	if !validSeverities[severity] {
		return fmt.Errorf("invalid severity: %s. Must be one of: INFO, WARN, ERR, CRIT", severity)
	}
	return nil
}

// validateBundleName validates the bundle name format
func validateBundleName(bundle string) error {
	if strings.TrimSpace(bundle) == "" {
		return fmt.Errorf("bundle name cannot be empty")
	}
	
	// Bundle names should be lowercase alphanumeric with underscores and hyphens
	// and should not start/end with special characters
	bundlePattern := regexp.MustCompile(`^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$`)
	if !bundlePattern.MatchString(bundle) {
		return fmt.Errorf("invalid bundle name: %s. Bundle names must be lowercase alphanumeric with underscores/hyphens, and cannot start or end with special characters", bundle)
	}
	
	return nil
}

var reportCommand = cmd.Command{
	Description: "Send a single report to the backend system.",
	Options: []cmd.Option{
		{
			Name:     reportSeverityOption,
			Short:    "s",
			Help:     "Severity of the report (INFO, WARN, ERR, CRIT).",
			Required: true,
		},
		{
			Name:     reportTextOption,
			Short:    "t",
			Help:     "Text summary of the report.",
			Required: true,
		},
		{
			Name:     reportBundleOption,
			Short:    "b",
			Help:     "Bundle name for the report.",
			Required: true,
		},
		{
			Name:  reportLabelsOption,
			Short: "l",
			Help:  "Labels for the report (comma-separated).",
		},
	},
	Target: func(opts cmd.Options) error {
		severity := opts[reportSeverityOption]
		text := opts[reportTextOption]
		bundle := opts[reportBundleOption]
		labels := opts[reportLabelsOption]

		// Validate severity
		if err := validateSeverity(severity); err != nil {
			return err
		}

		// Validate bundle name
		if err := validateBundleName(bundle); err != nil {
			return err
		}

		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		var deviceAgent *agent.Agent
		if deviceAgent, err = agent.New(cfg); err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		// Create the report
		report := configuration.Report{
			Bundle:         bundle,
			BundleCommitID: "",      // Empty for manual reports
			CommitID:       "",      // Empty for manual reports
			Labels:         labels,
			Severity:       severity,
			Text:           text,
			Log:            "",      // Empty for manual reports
			Timestamp:      time.Now().Unix(),
		}

		// Send the report using the configuration service
		reports := []configuration.Report{report}
		if err := deviceAgent.Configuration.SendReport(ctx, reports); err != nil {
			return err
		}

		fmt.Printf("Report sent successfully: [%s] %s\n", severity, text)
		return nil
	},
}