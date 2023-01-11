package configuration

import "context"

type Metadata struct {
	Enabled  bool   `json:"enabled"`
	Version  string `json:"version"`
	CommitID string `json:"bundle_commit_id"`
}

// BundleCommitID return bundle commit ID for the current bundle.
func (m Metadata) BundleCommitID() string {
	return m.CommitID
}

type Bundle interface {
	BundleCommitID() string
	Execute(context.Context, *Service) error
}
