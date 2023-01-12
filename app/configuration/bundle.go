package configuration

import "context"

type Metadata struct {
	Enabled  bool   `json:"enabled"`
	Version  string `json:"version"`
	CommitID string `json:"bundle_commit_id"`
}

// IsEnabled returns true if bundle is enabled
func (m Metadata) IsEnabled() bool {
	return m.Enabled
}

// BundleCommitID return bundle commit ID for the current bundle.
func (m Metadata) BundleCommitID() string {
	return m.CommitID
}

type Bundle interface {
	IsEnabled() bool
	BundleCommitID() string
	Execute(context.Context, *Service) error
}
