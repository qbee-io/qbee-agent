package configuration

import (
	"context"
	"errors"

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

	// connectivityWatchdogThreshold defines failed API connections threshold at which server will be rebooted
	// 0 -> disabled
	connectivityWatchdogThreshold int
	failedConnectionsCount        int
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
	// disable connectivity watchdog if not set in the configData
	if !configData.HasBundle(BundleConnectivityWatchdog) {
		srv.connectivityWatchdogThreshold = 0
	}

	reporter := NewReporter(configData.CommitID, srv.reportToConsole)

	for _, bundleName := range configData.Bundles {
		bundle := configData.selectBundleByName(bundleName)
		if bundle == nil {
			log.Errorf("configuration missing for bundle %s - skipping", bundleName)
			continue
		}

		if !bundle.IsEnabled() {
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

// reportAPIError tracks failed API connection attempts, so we can trigger reboot when connectivity watchdog is enabled.
func (srv *Service) reportAPIError(ctx context.Context, err error) {
	if srv.connectivityWatchdogThreshold == 0 {
		return
	}

	if !errors.As(err, new(api.ConnectionError)) {
		srv.failedConnectionsCount = 0
		return
	}

	srv.failedConnectionsCount++

	if srv.failedConnectionsCount >= srv.connectivityWatchdogThreshold {
		srv.RebootAfterRun(ctx)
	}
}
