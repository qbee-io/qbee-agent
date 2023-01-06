package configuration

import (
	"context"
	"encoding/base64"
	"fmt"
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

// Reporter is used to collect configuration reports from a single execution.
type Reporter struct {
	commitID string
	reports  []Report
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

// NewReporter returns a new instance of Reporter.
func NewReporter(commitID string) *Reporter {
	return &Reporter{
		commitID: commitID,
		reports:  make([]Report, 0),
	}
}

const (
	severityInfo     = "INFO"
	severityWarning  = "WARN"
	severityError    = "ERR"
	severityCritical = "CRIT"
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

// ReportCritical adds a critical message to the reporter instance set in context.
func ReportCritical(ctx context.Context, extraLog any, msgFmt string, args ...any) {
	addReport(ctx, severityCritical, extraLog, msgFmt, args...)
}

func addReport(ctx context.Context, severity string, extraLog any, msgFmt string, args ...any) {
	reporter, ok := ctx.Value(ctxReporter).(*Reporter)
	if !ok {
		return
	}

	var extraLogBytes []byte

	switch extraLogValue := extraLog.(type) {
	case string:
		extraLogBytes = []byte(extraLogValue)
	case []byte:
		extraLogBytes = extraLogValue
	case error:
		extraLogBytes = []byte(extraLogValue.Error())
	}

	report := Report{
		Bundle:         ctx.Value(ctxReporterBundleName).(string),
		BundleCommitID: ctx.Value(ctxReporterBundleCommitID).(string),
		CommitID:       reporter.commitID,
		Labels:         ctx.Value(ctxReporterBundleName).(string),
		Severity:       severity,
		Text:           fmt.Sprintf(msgFmt, args...),
		Log:            base64.StdEncoding.EncodeToString(extraLogBytes),
		Timestamp:      time.Now().Unix(),
	}

	reporter.reports = append(reporter.reports, report)
}
