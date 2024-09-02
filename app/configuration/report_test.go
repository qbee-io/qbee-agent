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

package configuration

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/utils/assert"
)

func Test_Reporter_Redact(t *testing.T) {
	cases := []struct {
		name               string
		secrets            []string
		testFn             func(ctx context.Context)
		expectedReportText string
		expectedReportLog  string
	}{
		{
			name: "reporter with no secrets",
			testFn: func(ctx context.Context) {
				ReportInfo(ctx, nil, "log message with secret123")
			},
			expectedReportText: "log message with secret123",
		},
		{
			name:    "reporter with secret in log message",
			secrets: []string{"secret123"},
			testFn: func(ctx context.Context) {
				ReportInfo(ctx, nil, "log message with secret123")
			},
			expectedReportText: "log message with ********",
		},
		{
			name:    "reporter with secret in log message arguments",
			secrets: []string{"secret123"},
			testFn: func(ctx context.Context) {
				ReportInfo(ctx, nil, "log message with %s", "secret123")
			},
			expectedReportText: "log message with ********",
		},
		{
			name:    "reporter with secret in extra log",
			secrets: []string{"secret123"},
			testFn: func(ctx context.Context) {
				ReportInfo(ctx, "recording secret123 in extra log", "log message")
			},
			expectedReportText: "log message",
			expectedReportLog:  "recording ******** in extra log",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reporter := NewReporter("", false, c.secrets)

			ctx := reporter.BundleContext(context.Background(), "", "")

			c.testFn(ctx)

			assert.Length(t, reporter.reports, 1)

			report := reporter.reports[0]

			assert.Equal(t, report.Text, c.expectedReportText)

			var extraLog string

			if report.Log != "" {
				extraLogBytes, err := base64.StdEncoding.DecodeString(report.Log)
				assert.NoError(t, err)

				extraLog = string(extraLogBytes)
			}

			assert.Equal(t, extraLog, c.expectedReportLog)

		})
	}
}

func Test_Reporter_Skip_Connectivity_Issues(t *testing.T) {
	cases := []struct {
		name                 string
		extraLog             any
		expectedReportLength int
	}{
		{
			name:                 "report connectivity issue",
			extraLog:             api.NewConnectionError(fmt.Errorf("connectivity issue")),
			expectedReportLength: 0,
		},
		{
			name:                 "report error",
			extraLog:             fmt.Errorf("error"),
			expectedReportLength: 1,
		},
		{
			name:                 "errpr with string",
			extraLog:             "this is a string",
			expectedReportLength: 1,
		},
		{
			name:                 "report error with bytes",
			extraLog:             []byte("this is a byte slice"),
			expectedReportLength: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reporter := NewReporter("", false, nil)

			ctx := reporter.BundleContext(context.Background(), "", "")

			ReportError(ctx, c.extraLog, "error message")

			assert.Length(t, reporter.reports, c.expectedReportLength)
		})
	}
}
