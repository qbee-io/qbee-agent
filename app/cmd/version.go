package cmd

import (
	"fmt"

	"qbee.io/platform/utils/cmd"

	"github.com/qbee-io/qbee-agent/app"
)

var versionCommand = cmd.Command{
	Description: "Agent version.",
	Target: func(opts cmd.Options) error {
		fmt.Printf("%s (commit: %s)\n", app.Version, app.Commit)
		return nil
	},
}
