package configuration

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

const defaultAgentInterval = 5                      // minutes
const reportsBufferExpiration = 10 * 24 * time.Hour // 10 days

type Service struct {
	api            *api.Client
	cacheDirectory string

	// currentConfig is the currently applied configuration
	currentConfig *CommittedConfig

	// configChangeTime represents a time when the currentCommitID changed last time
	configChangeTime time.Time

	rebootAfterRun           bool
	reportToConsole          bool
	reportingEnabled         bool
	metricsEnabled           bool
	remoteConsoleEnabled     bool
	softwareInventoryEnabled bool
	processInventoryEnabled  bool

	runInterval               int
	runIntervalChangeNotifier chan time.Duration

	// connectivityWatchdogThreshold defines failed API connections threshold at which server will be rebooted
	// 0 -> disabled
	connectivityWatchdogThreshold int
	failedConnectionsCount        int

	reportsBuffer     []Report
	reportsBufferLock sync.Mutex
}

// New returns a new instance of configuration Service.
func New(apiClient *api.Client, cacheDirectory string) *Service {
	return &Service{
		api:            apiClient,
		cacheDirectory: cacheDirectory,
		runInterval:    defaultAgentInterval,
		reportsBuffer:  make([]Report, 0),

		// this will notify the main agent loop about changes to the agent run interval
		// we don't expect more than a single consumer of this, that's why a buffered channel is used
		runIntervalChangeNotifier: make(chan time.Duration, 1),
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

// Current returns currently applied configuration.
func (srv *Service) Current() *CommittedConfig {
	return srv.currentConfig
}

// CurrentCommitID returns currently applied commit ID.
func (srv *Service) CurrentCommitID() string {
	if srv.Current() == nil {
		return ""
	}

	return srv.Current().CommitID
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
func (srv *Service) UpdateSettings(configData *CommittedConfig) {
	// if no enabled settings bundle available, use defaults
	if !configData.HasBundle(BundleSettings) || !configData.BundleData.Settings.Enabled {
		srv.applyDefaultSettings()
		return
	}

	configData.BundleData.Settings.Execute(srv)
}

// Execute configuration bundles on the system and return true if system should be rebooted.
func (srv *Service) Execute(ctx context.Context, configData *CommittedConfig) error {
	if err := acquireLock(); err != nil {
		log.Infof("failed to acquire execution lock - %v", err)
		return nil
	}
	defer releaseLock()

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
	if srv.CurrentCommitID() != configData.CommitID {
		srv.currentConfig = configData
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

// RunIntervalChangedNotifier returns a channel which will send a new agent interval duration when it changes.
func (srv *Service) RunIntervalChangedNotifier() <-chan time.Duration {
	return srv.runIntervalChangeNotifier
}

// addReportsToBuffer adds reports to the end of the delivery buffer.
func (srv *Service) addReportsToBuffer(reports []Report) {
	srv.reportsBufferLock.Lock()
	defer srv.reportsBufferLock.Unlock()

	// identify reports which are older than the buffer expiration
	reportsExpirationCutoff := time.Now().Add(-reportsBufferExpiration).Unix()

	i := 0
	for _, report := range srv.reportsBuffer {
		if report.Timestamp > reportsExpirationCutoff {
			break
		}

		i++
	}

	srv.reportsBuffer = append(srv.reportsBuffer[i:], reports...)
}
