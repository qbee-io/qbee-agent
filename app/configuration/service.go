package configuration

import (
	"context"
	"errors"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

const defaultAgentInterval = 5 // minutes

type Service struct {
	api            *api.Client
	cacheDirectory string

	// currentCommitID represents commit ID of committed device config currently applied to the system
	currentCommitID string

	// configChangeTime represents a time when the currentCommitID changed last time
	configChangeTime time.Time

	rebootAfterRun           bool
	reportToConsole          bool
	reportingEnabled         bool
	metricsEnabled           bool
	remoteConsoleEnabled     bool
	softwareInventoryEnabled bool
	processInventoryEnabled  bool

	runInterval int
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

// MetricsEnabled returns true if metrics collection is enabled.
func (srv *Service) MetricsEnabled() bool {
	return srv.metricsEnabled
}

// RemoteConsoleEnabled returns true if remote console access is enabled.
func (srv *Service) RemoteConsoleEnabled() bool {
	return srv.remoteConsoleEnabled
}

// CollectSoftwareInventory returns true if software inventory collection is enabled.
func (srv *Service) CollectSoftwareInventory() bool {
	return srv.softwareInventoryEnabled
}

// CollectProcessInventory returns true if process inventory collection is enabled.
func (srv *Service) CollectProcessInventory() bool {
	return srv.processInventoryEnabled
}

// RunInterval returns agent's run interval.
func (srv *Service) RunInterval() time.Duration {
	return time.Duration(srv.runInterval) * time.Minute
}

// CurrentCommitID returns currently applied commit ID.
func (srv *Service) CurrentCommitID() string {
	return srv.currentCommitID
}

// ConfigChangeTimestamp returns timestamp when the configuration commit ID changed last time.
func (srv *Service) ConfigChangeTimestamp() int64 {
	return srv.configChangeTime.Unix()
}

// EnableConsoleReporting enables reporting to console.
func (srv *Service) EnableConsoleReporting() {
	srv.reportToConsole = true
}

// applyDefaultSettings to the agent.
func (srv *Service) applyDefaultSettings() {
	srv.reportToConsole = true
	srv.reportingEnabled = true
	srv.metricsEnabled = true
	srv.remoteConsoleEnabled = true
	srv.softwareInventoryEnabled = true
	srv.processInventoryEnabled = false
	srv.runInterval = defaultAgentInterval
}

// UpdateSettings of the agent based on provided config data.
func (srv *Service) UpdateSettings(ctx context.Context, configData *CommittedConfig) error {
	// if no enabled settings bundle available, use defaults
	if !configData.HasBundle(BundleSettings) || !configData.BundleData.Settings.Enabled {
		srv.applyDefaultSettings()
		return nil
	}

	return configData.BundleData.Settings.Execute(ctx, srv)
}

// Execute configuration bundles on the system and return true if system should be rebooted.
func (srv *Service) Execute(ctx context.Context, configData *CommittedConfig) error {
	// disable connectivity watchdog if not set in the configData
	if !configData.HasBundle(BundleConnectivityWatchdog) {
		srv.connectivityWatchdogThreshold = 0
	}

	reporter := NewReporter(configData.CommitID, srv.reportToConsole)

	for _, bundleName := range configData.Bundles {
		// we use srv.UpdateSettings method to execute the settings bundle
		if bundleName == BundleSettings {
			continue
		}

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
		}
	}

	// assign config's commitID as current
	if srv.currentCommitID != configData.CommitID {
		srv.currentCommitID = configData.CommitID
		srv.configChangeTime = time.Now()
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
