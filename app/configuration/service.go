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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/metrics"
	"go.qbee.io/agent/app/utils/files"
)

const defaultAgentInterval = 5 // minutes
const minimumAgentInterval = 5 // minute
const defaultFirstRunRetryCounter = 5

const (
	// defaultExecutionTimeout is the time configuration execution may take, unless set in the settings bundle.
	defaultExecutionTimeout = time.Hour

	// minimumExecutionTimeout is the smallest execution timeout accepted from the settings bundle.
	minimumExecutionTimeout = 5 * time.Minute

	// maximumExecutionTimeout is the largest execution timeout accepted from the settings bundle.
	// It also limits how long signed URLs are valid (see Agent.signedURLTTL).
	maximumExecutionTimeout = 24 * time.Hour
)

// Service provides configuration management functionality for the agent.
type Service struct {
	api *api.Client

	// appDirectory is a directory where the agent stores its data
	appDirectory string

	// cacheDirectory is a directory where the agent stores its cache data
	cacheDirectory string

	// userCacheDirectory is a directory where the agent stores its non-root user cache data
	userCacheDirectory string

	// commitMutex guards currentCommitID and configChangeTime, which are written by configuration execution
	// and read during check-in.
	commitMutex sync.RWMutex

	// currentCommitID represents commit ID of committed device config currently applied to the system
	currentCommitID string

	// configChangeTime represents a time when the currentCommitID changed last time
	configChangeTime time.Time

	// urlSigner is used to sign URLs for the device hub
	urlSigner URLSigner

	// lockFile overrides the path of the configuration execution lock file (used in tests)
	lockFile string

	// Fields of atomic types are accessed by both check-in and configuration execution, which run concurrently.
	// Other fields are not accessed by configuration execution.
	rebootAfterRun           atomic.Bool
	reportToConsole          bool
	reportingEnabled         atomic.Bool
	metricsEnabled           bool
	softwareInventoryEnabled atomic.Bool
	processInventoryEnabled  atomic.Bool

	runInterval               int
	runIntervalChangeNotifier chan time.Duration

	// executionTimeout defines the time limit of the configuration execution
	executionTimeout atomic.Int64

	// activeExecutionTimeout is the time limit of the configuration execution in progress (0 if none).
	// It may differ from executionTimeout, when settings changed since the execution started.
	activeExecutionTimeout atomic.Int64

	// connectivityWatchdogThreshold defines failed API connections threshold at which server will be rebooted
	// 0 -> disabled
	connectivityWatchdogThreshold atomic.Int64
	failedConnectionsCount        int

	// metrics service
	metrics *metrics.Service

	// firstRun is true if the agent is running for the first time after startup
	firstRunRetryCounter int

	// configEndpointUnreachable is true if the config endpoint is unreachable
	configEndpointUnreachable atomic.Bool

	// reportsBufferMutex serializes access to the reports buffer file
	reportsBufferMutex sync.Mutex
}

// New returns a new instance of configuration Service.
func New(apiClient *api.Client, appDirectory, cacheDirectory string) *Service {
	srv := &Service{
		api:            apiClient,
		appDirectory:   appDirectory,
		cacheDirectory: cacheDirectory,
		runInterval:    defaultAgentInterval,

		// this will notify the main agent loop about changes to the agent run interval
		// we don't expect more than a single consumer of this, that's why a buffered channel is used
		runIntervalChangeNotifier: make(chan time.Duration, 1),
		firstRunRetryCounter:      defaultFirstRunRetryCounter,
	}

	srv.executionTimeout.Store(int64(defaultExecutionTimeout))

	return srv
}

// WithURLSigner sets the URL signer for the service.
func (srv *Service) WithURLSigner(urlSigner URLSigner) *Service {
	srv.urlSigner = urlSigner
	return srv
}

// WithLockFile sets the path of the configuration execution lock file.
// By default, the lock file is shared by all agent processes on the device. Tests use their own lock file,
// so they don't block each other when running in parallel.
func (srv *Service) WithLockFile(path string) *Service {
	srv.lockFile = path
	return srv
}

// WithUserCacheDirectory sets the user cache directory for the service.
func (srv *Service) WithUserCacheDirectory(userCacheDirectory string) *Service {
	srv.userCacheDirectory = userCacheDirectory
	return srv
}

// WithMetricsService sets the metrics service for the service.
func (srv *Service) WithMetricsService(metricsService *metrics.Service) *Service {
	srv.metrics = metricsService
	return srv
}

// MetricsEnabled returns true if metrics collection is enabled.
func (srv *Service) MetricsEnabled() bool {
	return srv.metricsEnabled
}

// ReportingEnabled returns true if reporting is enabled.
func (srv *Service) ReportingEnabled() bool {
	return srv.reportingEnabled.Load()
}

// CollectSoftwareInventory returns true if software inventory collection is enabled.
func (srv *Service) CollectSoftwareInventory() bool {
	return srv.softwareInventoryEnabled.Load()
}

// CollectProcessInventory returns true if process inventory collection is enabled.
func (srv *Service) CollectProcessInventory() bool {
	return srv.processInventoryEnabled.Load()
}

// RunInterval returns agent's run interval.
func (srv *Service) RunInterval() time.Duration {
	return time.Duration(srv.runInterval) * time.Minute
}

// ExecutionTimeout returns the time limit of the configuration execution.
func (srv *Service) ExecutionTimeout() time.Duration {
	return time.Duration(srv.executionTimeout.Load())
}

// ActiveExecutionTimeout returns the time limit of the configuration execution in progress,
// or 0 if no configuration execution is in progress.
func (srv *Service) ActiveExecutionTimeout() time.Duration {
	return time.Duration(srv.activeExecutionTimeout.Load())
}

// setExecutionTimeout sets the time limit of the configuration execution.
// Non-positive value resets it to the default, values outside the allowed range are raised or lowered to it.
func (srv *Service) setExecutionTimeout(minutes int) {
	// the value is checked before it's converted to time.Duration, which overflows for very large values
	var timeout time.Duration

	switch {
	case minutes <= 0:
		timeout = defaultExecutionTimeout
	case minutes < int(minimumExecutionTimeout/time.Minute):
		timeout = minimumExecutionTimeout
	case minutes > int(maximumExecutionTimeout/time.Minute):
		timeout = maximumExecutionTimeout
	default:
		timeout = time.Duration(minutes) * time.Minute
	}

	srv.executionTimeout.Store(int64(timeout))
}

// CurrentCommitID returns currently applied commit ID.
func (srv *Service) CurrentCommitID() string {
	srv.commitMutex.RLock()
	defer srv.commitMutex.RUnlock()

	return srv.currentCommitID
}

// ConfigChangeTimestamp returns timestamp when the configuration commit ID changed last time.
func (srv *Service) ConfigChangeTimestamp() int64 {
	srv.commitMutex.RLock()
	defer srv.commitMutex.RUnlock()

	return srv.configChangeTime.Unix()
}

// EnableConsoleReporting enables reporting to console.
func (srv *Service) EnableConsoleReporting() {
	srv.reportToConsole = true
}

// applyDefaultSettings to the agent.
func (srv *Service) applyDefaultSettings() {
	srv.reportingEnabled.Store(true)
	srv.metricsEnabled = true
	srv.softwareInventoryEnabled.Store(true)
	srv.processInventoryEnabled.Store(false)
	srv.runInterval = defaultAgentInterval
	srv.setExecutionTimeout(0)
	setPkgCacheTTL(0)
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

// UpdateMetricsMonitorState deletes the monitor state if the metrics monitor bundle is disabled
// or not configured in the provided config data.
func (srv *Service) UpdateMetricsMonitorState(configData *CommittedConfig) {
	if !configData.HasBundle(BundleMetricsMonitor) || !configData.BundleData.MetricsMonitor.Enabled {
		deleteMetricsMonitorState()
	}
}

// Execute configuration bundles on the system.
// Execution is limited by the configured execution timeout.
func (srv *Service) Execute(ctx context.Context, configData *CommittedConfig) error {
	log.Debugf("trying to acquire execution lock")

	executionTimeout := srv.ExecutionTimeout()

	parametersBundle := configData.BundleData.Parameters
	if parametersBundle == nil {
		parametersBundle = new(ParametersBundle)
	}
	ctxWithParameters := parametersBundle.Context(ctx, srv.urlSigner)

	ctxWithTimeout, cancel := context.WithTimeout(ctxWithParameters, executionTimeout)
	defer cancel()

	if err := srv.acquireLock(executionTimeout); err != nil {
		log.Infof("failed to acquire execution lock - %v", err)
		return nil
	}
	defer func() {
		if err := srv.releaseLock(); err != nil {
			log.Errorf("failed to release execution lock - %v", err)
		}
	}()

	srv.activeExecutionTimeout.Store(int64(executionTimeout))
	defer srv.activeExecutionTimeout.Store(0)

	// disable connectivity watchdog if not set in the configData
	if !configData.HasBundle(BundleConnectivityWatchdog) {
		log.Debugf("connectivity watchdog bundle not found - disabling watchdog")
		srv.connectivityWatchdogThreshold.Store(0)
	}

	reporter := NewReporter(configData.CommitID, srv.reportToConsole, configData.SecretsList(ctxWithParameters))

	for _, bundleName := range configData.Bundles {
		log.Debugf("starting processing of bundle %s", bundleName)

		// Stop bundles execution if execution time limit was reached.
		if ctxWithTimeout.Err() != nil {
			log.Warnf("configuration execution exceeded time limit of %s - remaining bundles will be applied during "+
				"the next run", executionTimeout)

			internalCtx := reporter.BundleContext(ctx, bundleAgentInternal, "")
			ReportWarning(internalCtx, nil,
				"Configuration execution exceeded time limit of %s. Remaining bundles will be applied during the next run.",
				executionTimeout)
			break
		}

		switch bundleName {
		case BundleSettings:
			// we use srv.UpdateSettings method to execute the settings bundle
			log.Debugf("skipping settings bundle execution, as it's processed separately")
			continue
		case BundleParameters:
			log.Debugf("skipping parameters bundle execution, as it's processed separately")
			continue
		}

		bundle := configData.selectBundleByName(bundleName)
		if bundle == nil {
			log.Errorf("configuration missing for bundle %s - skipping", bundleName)
			continue
		}

		if !bundle.IsEnabled() {
			log.Debugf("bundle %s is disabled - skipping", bundleName)
			continue
		}

		bundleCtx := reporter.BundleContext(ctxWithTimeout, bundleName, bundle.BundleCommitID())

		log.Debugf("executing bundle %s", bundleName)
		if err := bundle.Execute(bundleCtx, srv); err != nil {
			log.Errorf("bundle %s execution failed: %v", bundleName, err)
		}

		log.Debugf("bundle %s execution finished", bundleName)
	}

	// assign config's commitID as current
	srv.commitMutex.Lock()
	if srv.currentCommitID != configData.CommitID {
		log.Debugf("updating current commit ID to %s", configData.CommitID)
		srv.currentCommitID = configData.CommitID
		srv.configChangeTime = time.Now()
	}
	srv.commitMutex.Unlock()

	if !srv.ReportingEnabled() {
		log.Debugf("reporting is disabled - skipping sending reports")
		return nil
	}

	log.Debugf("sending reports to the server")
	if _, err := srv.sendReports(ctx, reporter.Reports()); err != nil {
		log.Debugf("failed to send reports to the server: %v, adding to the buffer", err)

		if bufferErr := srv.addReportsToBuffer(reporter.Reports()); bufferErr != nil {
			log.Errorf("failed to add reports to buffer: %v", bufferErr)
		}

		return err
	}

	log.Debugf("attempting to flush reports buffer")

	if err := srv.flushReportsBuffer(ctx); err != nil {
		log.Errorf("failed to flush reports buffer: %v", err)
	}

	return nil
}

// RebootAfterRun schedules system reboot after current agent run.
func (srv *Service) RebootAfterRun(ctx context.Context) {
	if !srv.rebootAfterRun.CompareAndSwap(false, true) {
		return
	}

	ReportWarning(ctx, nil, "Scheduling system reboot.")
}

// ShouldReboot returns true if system should be restarted after agent run.
func (srv *Service) ShouldReboot() bool {
	return srv.rebootAfterRun.Load()
}

// reportAPIError tracks failed API connection attempts, so we can trigger reboot when connectivity watchdog is enabled.
func (srv *Service) reportAPIError(ctx context.Context, err error) {
	// read once, as configuration execution may change it concurrently
	threshold := srv.connectivityWatchdogThreshold.Load()
	if threshold == 0 {
		return
	}

	if !errors.As(err, new(api.ConnectionError)) {
		srv.failedConnectionsCount = 0
		return
	}

	srv.failedConnectionsCount++

	if int64(srv.failedConnectionsCount) >= threshold {
		// since we don't have a reporter defined on this context, we need to create a new one
		reporter := NewReporter(srv.CurrentCommitID(), srv.reportToConsole, nil)
		bundleCtx := reporter.BundleContext(ctx, BundleConnectivityWatchdog, "")

		srv.RebootAfterRun(bundleCtx)

		// Since we are reporting API issue, there is probably no point sending the reports,
		// so we just add them straight to the buffer on the filesystem.
		// They will be delivered on the next successful run.
		if err := srv.addReportsToBuffer(reporter.Reports()); err != nil {
			log.Errorf("failed to add reports to buffer: %v", err)
		}
	}
}

// ReportExhaustedGoroutineBudget reports when the goroutine budget is exhausted and returns whether the
// budget was exhausted and any error encountered delivering the report.
func (srv *Service) ReportExhaustedGoroutineBudget(ctx context.Context) (bool, error) {
	if !files.GoroutinesExhausted() {
		return false, nil
	}

	// since we don't have a reporter defined on this context, we need to create a new one
	reporter := NewReporter(srv.CurrentCommitID(), srv.reportToConsole, nil)
	bundleCtx := reporter.BundleContext(ctx, bundleAgentInternal, "")

	ReportCritical(bundleCtx, nil, "goroutine budget exhausted, skipping agent run")

	// send reports immediately as there will be no further configurations executed, thus
	// no further reports will be generated. If we fail to send the reports, we add them to the buffer.
	if _, err := srv.sendReports(ctx, reporter.Reports()); err != nil {
		log.Debugf("failed to send reports to the server: %v, adding to the buffer", err)

		if bufferErr := srv.addReportsToBuffer(reporter.Reports()); bufferErr != nil {
			log.Errorf("failed to add reports to buffer: %v", bufferErr)
		}

		return true, err
	}

	// also flush any existing buffered reports
	if err := srv.flushReportsBuffer(ctx); err != nil {
		log.Errorf("failed to flush reports buffer: %v", err)
	}

	return true, nil
}

// ReportRemoteAccessError reports a remote access connection error with the error string as extra log.
// reportingEnabled and commitID are passed in, as this is called outside the agent run.
func (srv *Service) ReportRemoteAccessError(ctx context.Context, err error, reportingEnabled bool, commitID string) error {
	if !reportingEnabled {
		return nil
	}

	// since we don't have a reporter defined on this context, we need to create a new one
	reporter := NewReporter(commitID, srv.reportToConsole, nil)
	bundleCtx := reporter.BundleContext(ctx, bundleAgentInternal, "")

	ReportError(bundleCtx, err, "Remote access connection failed.")

	// send reports immediately, as this is called outside the agent run.
	// If we fail to send the reports, we add them to the buffer.
	if _, err := srv.sendReports(ctx, reporter.Reports()); err != nil {
		log.Debugf("failed to send reports to the server: %v, adding to the buffer", err)

		if bufferErr := srv.addReportsToBuffer(reporter.Reports()); bufferErr != nil {
			log.Errorf("failed to add reports to buffer: %v", bufferErr)
		}

		return err
	}

	// also flush any existing buffered reports
	if err := srv.flushReportsBuffer(ctx); err != nil {
		log.Errorf("failed to flush reports buffer: %v", err)
	}

	return nil
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
	srv.reportsBufferMutex.Lock()
	defer srv.reportsBufferMutex.Unlock()

	return srv.writeReportsToBuffer(reports)
}

// writeReportsToBuffer appends reports to the delivery buffer. Caller must hold reportsBufferMutex.
func (srv *Service) writeReportsToBuffer(reports []Report) error {
	reportsBufferFilePath := filepath.Join(srv.appDirectory, reportsBufferFileName)

	fp, err := os.OpenFile(reportsBufferFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, reportsBufferFileMode)
	if err != nil {
		return fmt.Errorf("failed to open reports buffer file: %v", err)
	}
	defer func() { _ = fp.Close() }()

	encoder := json.NewEncoder(fp)

	for _, report := range reports {
		if err := encoder.Encode(report); err != nil {
			return fmt.Errorf("failed to encode report: %v", err)
		}
	}

	// sync disk writes to avoid data loss
	if err = fp.Sync(); err != nil {
		return fmt.Errorf("failed to sync reports buffer file: %v", err)
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
	defer func() { _ = fp.Close() }()

	var reports []Report
	decoder := json.NewDecoder(fp)

	reportsExpirationCutoff := time.Now().Add(-reportsBufferExpiration).Unix()

	for decoder.More() {
		var offset int64
		var report Report
		if err := decoder.Decode(&report); err != nil {

			var serr *json.SyntaxError
			var ok bool
			if serr, ok = err.(*json.SyntaxError); !ok {
				log.Errorf("failed to decode report: %v", err)
				return reports, nil
			}

			offset = offset + serr.Offset
			log.Errorf("syntax error at offset %d, trying to recover: %v", offset, err)
			buffer, err := io.ReadAll(decoder.Buffered())

			if err != nil {
				log.Errorf("readall error: %v\n", err)
				return reports, nil
			}

			if serr.Offset > int64(len(buffer)) {
				log.Errorf("offset %d is beyond buffer length %d", serr.Offset, len(buffer))
				return reports, nil
			}

			buffer = buffer[serr.Offset:]
			decoder = json.NewDecoder(io.MultiReader(bytes.NewBuffer(buffer), fp))
			continue
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
	srv.reportsBufferMutex.Lock()
	defer srv.reportsBufferMutex.Unlock()

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
		if err = srv.writeReportsToBuffer(reports[delivered:]); err != nil {
			log.Errorf("failed to re-add reports to buffer: %v", err)
		}
	}

	return nil
}

// Get returns the agent configuration.
// If the configuration cannot be retrieved from the API, it will be loaded from the local cache.
func (srv *Service) Get(ctx context.Context, fresh bool) (*CommittedConfig, error) {
	cfg, err := srv.get(ctx, fresh)
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
	defer func() { _ = fp.Close() }()

	if err = json.NewEncoder(fp).Encode(cfg); err != nil {
		log.Errorf("failed to marshal config: %v", err)
		return
	}

	// sync disk writes to avoid data loss
	if err = fp.Sync(); err != nil {
		log.Errorf("failed to sync config file: %v", err)
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
	defer func() { _ = fp.Close() }()

	if err = json.NewDecoder(fp).Decode(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return nil
}

// IsConfigEndpointUnreachable returns true if the configuration endpoint is unreachable.
func (srv *Service) IsConfigEndpointUnreachable() bool {
	return srv.configEndpointUnreachable.Load()
}
