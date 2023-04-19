package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

const defaultAgentInterval = 5 // minutes

type Service struct {
	api *api.Client

	// appDirectory is a directory where the agent stores its data
	appDirectory string

	// cacheDirectory is a directory where the agent stores its cache data
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

	runInterval               int
	runIntervalChangeNotifier chan time.Duration

	// connectivityWatchdogThreshold defines failed API connections threshold at which server will be rebooted
	// 0 -> disabled
	connectivityWatchdogThreshold int
	failedConnectionsCount        int
}

// New returns a new instance of configuration Service.
func New(apiClient *api.Client, appDirectory, cacheDirectory string) *Service {
	return &Service{
		api:            apiClient,
		appDirectory:   appDirectory,
		cacheDirectory: cacheDirectory,
		runInterval:    defaultAgentInterval,

		// this will notify the main agent loop about changes to the agent run interval
		// we don't expect more than a single consumer of this, that's why a buffered channel is used
		runIntervalChangeNotifier: make(chan time.Duration, 1),
	}
}

// MetricsEnabled returns true if metrics collection is enabled.
func (srv *Service) MetricsEnabled() bool {
	return srv.metricsEnabled
}

// RemoteAccessEnabled returns true if remote access is enabled.
func (srv *Service) RemoteAccessEnabled() bool {
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
	if srv.currentCommitID != configData.CommitID {
		srv.currentCommitID = configData.CommitID
		srv.configChangeTime = time.Now()
	}

	if srv.reportingEnabled {
		if _, err := srv.sendReports(ctx, reporter.Reports()); err != nil {
			if bufferErr := srv.addReportsToBuffer(reporter.Reports()); bufferErr != nil {
				log.Errorf("failed to add reports to buffer: %v", bufferErr)
			}

			return err
		}

		// attempt to flush reports buffer if reports were sent successfully
		if err := srv.flushReportsBuffer(ctx); err != nil {
			log.Errorf("failed to flush reports buffer: %v", err)
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

const (
	reportsBufferFileName   = "reports.jsonl"
	reportsBufferFileMode   = 0600
	reportsBufferExpiration = 30 * 24 * time.Hour
)

// addReportsToBuffer adds reports to the delivery buffer.
func (srv *Service) addReportsToBuffer(reports []Report) error {
	reportsBufferFilePath := filepath.Join(srv.appDirectory, reportsBufferFileName)

	fp, err := os.OpenFile(reportsBufferFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, reportsBufferFileMode)
	if err != nil {
		return fmt.Errorf("failed to open reports buffer file: %v", err)
	}
	defer fp.Close()

	encoder := json.NewEncoder(fp)

	for _, report := range reports {
		if err := encoder.Encode(report); err != nil {
			return fmt.Errorf("failed to encode report: %v", err)
		}
	}

	return nil
}

// readReportsBuffer reads reports from the delivery buffer.
func (srv *Service) readReportsBuffer() ([]Report, error) {
	reportsBufferFilePath := filepath.Join(srv.appDirectory, reportsBufferFileName)

	fp, err := os.Open(reportsBufferFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to open reports buffer file: %v", err)
	}
	defer fp.Close()

	var reports []Report
	decoder := json.NewDecoder(fp)

	reportsExpirationCutoff := time.Now().Add(-reportsBufferExpiration).Unix()

	for {
		var report Report
		if err := decoder.Decode(&report); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("failed to decode report: %v", err)
		}

		// don't return reports that are too old
		if report.Timestamp < reportsExpirationCutoff {
			continue
		}

		reports = append(reports, report)
	}

	return reports, nil
}

// clearReportsBuffer removes reports from the delivery buffer.
func (srv *Service) clearReportsBuffer() error {
	if err := os.Remove(filepath.Join(srv.appDirectory, reportsBufferFileName)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to remove reports buffer file: %v", err)
	}

	return nil
}

// flushReportsBuffer attempts to send reports from the delivery buffer.
func (srv *Service) flushReportsBuffer(ctx context.Context) error {
	// load all undelivered reports from the buffer
	reports, err := srv.readReportsBuffer()
	if err != nil {
		return err
	}

	// if there are no reports to send, we're done
	if len(reports) == 0 {
		return nil
	}

	// try to send all reports
	delivered, deliveryErr := srv.sendReports(ctx, reports)

	// if we failed to send any reports, return error immediately
	if deliveryErr != nil && delivered == 0 {
		return deliveryErr
	}

	// if we delivered any reports, clear the buffer
	if err = srv.clearReportsBuffer(); err != nil {
		log.Errorf("failed to clear reports buffer: %v", err)
	}

	// if we failed to deliver some reports, re-add them to the buffer
	if deliveryErr != nil {
		if err = srv.addReportsToBuffer(reports[delivered:]); err != nil {
			log.Errorf("failed to re-add reports to buffer: %v", err)
		}
	}

	return nil
}

// Get returns the agent configuration.
// If the configuration cannot be retrieved from the API, it will be loaded from the local cache.
func (srv *Service) Get(ctx context.Context) (*CommittedConfig, error) {
	cfg, err := srv.get(ctx)
	if err != nil {
		// if we failed to get config from API, try to load it from local file cache
		cfg = new(CommittedConfig)

		if loadErr := srv.loadConfig(cfg); loadErr != nil {
			log.Warnf("failed to load config from cache: %v", loadErr)
			return nil, err
		}

		log.Warnf("failed to get config from API: %v", err)

		return cfg, nil
	}

	srv.persistConfig(cfg)

	return cfg, nil
}

const (
	configCacheFileName = "config.json"
	configCacheFileMode = 0600
)

// persistConfig saves the agent configuration to the cache file.
func (srv *Service) persistConfig(cfg *CommittedConfig) {
	filename := filepath.Join(srv.appDirectory, configCacheFileName)

	fp, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, configCacheFileMode)
	if err != nil {
		log.Errorf("failed to open config cache file: %v", err)
		return
	}
	defer fp.Close()

	if err = json.NewEncoder(fp).Encode(cfg); err != nil {
		log.Errorf("failed to marshal config: %v", err)
		return
	}
}

// loadConfig loads the agent configuration from the cache file.
func (srv *Service) loadConfig(cfg *CommittedConfig) error {
	filename := filepath.Join(srv.appDirectory, configCacheFileName)

	fp, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open config cache file: %v", err)
	}
	defer fp.Close()

	if err = json.NewDecoder(fp).Decode(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return nil
}
