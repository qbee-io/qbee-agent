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

// Execute configuration bundles on the system.
func (srv *Service) Execute(ctx context.Context, configData *CommittedConfig) error {
	reporter := NewReporter(configData.CommitID)

	for _, bundleName := range configData.Bundles {
		bundle := configData.selectBundleByName(bundleName)
		if bundle == nil {
			log.Errorf("configuration missing for bundle %s - skipping", bundleName)
			continue
		}

		bundleCtx := reporter.BundleContext(ctx, bundleName, bundle.BundleCommitID())

		if err := bundle.Execute(bundleCtx, srv, configData); err != nil {
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
