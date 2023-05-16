package cmd

import (
	"fmt"

	"github.com/qbee-io/qbee-agent/app"
)

var versionCommand = Command{
	Description: "Agent version.",
	Target: func(opts Options) error {
		fmt.Printf("%s (commit: %s)\n", app.Version, app.Commit)
		return nil
	},
}
