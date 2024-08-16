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
	"strings"
	"time"

	"go.qbee.io/agent/app/api"
)

// Report represents a single configuration report.
// The report will show in the Logs section of the platform.
type Report struct {
	Bundle         string `json:"bundle"`
	BundleCommitID string `json:"bundle_commit_id"`
	CommitID       string `json:"commit_id"`
	Labels         string `json:"labels"`

	// Severity of the report. Can be INFO, ERR, WARN, CRIT.
	Severity string `json:"sev"`

	// Text summary of the report.
	Text string `json:"text"`

	// Log contains base64-encoded operation log.
	Log string `json:"log,omitempty"`

	// Timestamp when the report was created.
	Timestamp int64 `json:"ts"`
}

func (report Report) String() string {
	return fmt.Sprintf("[%s] %s", report.Severity, report.Text)
}

// Reporter is used to collect configuration reports from a single execution.
type Reporter struct {
	commitID        string
	reports         []Report
	reportToConsole bool
	secrets         []string
}

type contextKey string

const (
	ctxReporter               = contextKey("configuration:reporter")
	ctxReporterBundleName     = contextKey("configuration:reporter:bundle-name")
	ctxReporterBundleCommitID = contextKey("configuration:reporter:bundle-commit-id")
)

// BundleContext returns context with bundle information attached to it.
func (reporter *Reporter) BundleContext(ctx context.Context, bundleName string, bundleCommitID string) context.Context {
	ctx = context.WithValue(ctx, ctxReporter, reporter)
	ctx = context.WithValue(ctx, ctxReporterBundleName, bundleName)
	return context.WithValue(ctx, ctxReporterBundleCommitID, bundleCommitID)
}

// Reports returns collected reports.
func (reporter *Reporter) Reports() []Report {
	return reporter.reports
}

const redactedValue = "********"

// Redact replaces all secrets in the value with redactedValue.
func (reporter *Reporter) Redact(value string) string {
	for _, secret := range reporter.secrets {
		value = strings.ReplaceAll(value, secret, redactedValue)
	}

	return value
}

// NewReporter returns a new instance of Reporter.
func NewReporter(commitID string, reportToConsole bool, secrets []string) *Reporter {
	return &Reporter{
		commitID:        commitID,
		reports:         make([]Report, 0),
		reportToConsole: reportToConsole,
		secrets:         secrets,
	}
}

const (
	severityInfo    = "INFO"
	severityWarning = "WARN"
	severityError   = "ERR"
)

// msgWithLabel returns a message with a label (if provided).
func msgWithLabel(label, msgFmt string, args ...any) string {
	if label == "" {
		return fmt.Sprintf(msgFmt, args...)
	}

	return fmt.Sprintf("[%s] %s", label, fmt.Sprintf(msgFmt, args...))
}

// ReportInfo adds an info message to the reporter instance set in context.
func ReportInfo(ctx context.Context, extraLog any, msgFmt string, args ...any) {
	addReport(ctx, severityInfo, extraLog, msgFmt, args...)
}

// ReportWarning adds a warning message to the reporter instance set in context.
func ReportWarning(ctx context.Context, extraLog any, msgFmt string, args ...any) {
	addReport(ctx, severityWarning, extraLog, msgFmt, args...)
}

// ReportError adds an error message to the reporter instance set in context.
func ReportError(ctx context.Context, extraLog any, msgFmt string, args ...any) {

	if _, skipLogging := extraLog.(api.ConnectionError); skipLogging {
		return
	}

	addReport(ctx, severityError, extraLog, msgFmt, args...)
}

const (
	consolePrefixReport = "report:"
	consolePrefixLog    = "log:"
)

func addReport(ctx context.Context, severity string, extraLog any, msgFmt string, args ...any) {
	reporter, ok := ctx.Value(ctxReporter).(*Reporter)
	if !ok {
		return
	}

	var extraLogBytes string

	switch extraLogValue := extraLog.(type) {
	case string:
		extraLogBytes = extraLogValue
	case []byte:
		extraLogBytes = string(extraLogValue)
	case error:
		extraLogBytes = extraLogValue.Error()
	}

	extraLogBytes = reporter.Redact(extraLogBytes)
	text := reporter.Redact(fmt.Sprintf(msgFmt, args...))

	report := Report{
		Bundle:         ctx.Value(ctxReporterBundleName).(string),
		BundleCommitID: ctx.Value(ctxReporterBundleCommitID).(string),
		CommitID:       reporter.commitID,
		Labels:         ctx.Value(ctxReporterBundleName).(string),
		Severity:       severity,
		Text:           text,
		Log:            base64.StdEncoding.EncodeToString([]byte(extraLogBytes)),
		Timestamp:      time.Now().Unix(),
	}

	if reporter.reportToConsole {
		if len(extraLogBytes) > 0 {
			for _, line := range strings.Split(strings.TrimSpace(extraLogBytes), "\n") {
				fmt.Println(consolePrefixLog, line)
			}
		}

		fmt.Println(consolePrefixReport, report)
	}

	reporter.reports = append(reporter.reports, report)
}
