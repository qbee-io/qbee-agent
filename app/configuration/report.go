package configuration

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

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

const (
	ctxReporter               = "configuration:reporter"
	ctxReporterBundleName     = "configuration:reporter:bundle-name"
	ctxReporterBundleCommitID = "configuration:reporter:bundle-commit-id"
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
