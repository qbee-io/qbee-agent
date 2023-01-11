package configuration

import (
	"context"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

type Service struct {
	api             *api.Client
	cacheDirectory  string
	currentCommitID string

	rebootAfterRun bool

	reportToConsole          bool
	reportingEnabled         bool
	metricsEnabled           bool
	remoteConsoleEnabled     bool
	softwareInventoryEnabled bool
	processInventoryEnabled  bool
	runInterval              int
}

// New returns a new instance of configuration Service.
func New(apiClient *api.Client, cacheDirectory string) *Service {
	return &Service{
		api:            apiClient,
		cacheDirectory: cacheDirectory,
	}
}

// CurrentCommitID returns currently applied commit ID.
func (srv *Service) CurrentCommitID() string {
	return srv.currentCommitID
}

// EnableConsoleReporting enables reporting to console.
func (srv *Service) EnableConsoleReporting() {
	srv.reportToConsole = true
}

// Execute configuration bundles on the system and return true if system should be rebooted.
func (srv *Service) Execute(ctx context.Context, configData *CommittedConfig) error {
	reporter := NewReporter(configData.CommitID, srv.reportToConsole)

	for _, bundleName := range configData.Bundles {
		bundle := configData.selectBundleByName(bundleName)
		if bundle == nil {
			log.Errorf("configuration missing for bundle %s - skipping", bundleName)
			continue
		}

		bundleCtx := reporter.BundleContext(ctx, bundleName, bundle.BundleCommitID())

		if err := bundle.Execute(bundleCtx, srv); err != nil {
			log.Errorf("bundle %s execution failed: %v", bundleName, err)
			break
		}
	}

	if srv.reportingEnabled {
		if err := srv.sendReports(ctx, reporter.Reports()); err != nil {
			return err
		}
	}

	return nil
}

// RebootAfterRun schedules system reboot after current agent run.
func (srv *Service) RebootAfterRun(ctx context.Context) {
	if srv.rebootAfterRun {
		return
	}

	ReportWarning(ctx, nil, "Scheduling system reboot.")
	srv.rebootAfterRun = true
}

// ShouldReboot returns true if system should be restarted after agent run.
func (srv *Service) ShouldReboot() bool {
	return srv.rebootAfterRun
}
