package configuration

import "context"

type Metadata struct {
	Enabled  bool   `json:"enabled"`
	Version  string `json:"version"`
	CommitID string `json:"bundle_commit_id"`
}

type Bundle interface {
	BundleCommitID(*CommittedConfig) string
	Execute(context.Context, *Service, *CommittedConfig) error
}
