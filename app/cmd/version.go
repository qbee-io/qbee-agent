package cmd

import (
	"fmt"

	"qbee.io/platform/utils/flags"

	"github.com/qbee-io/qbee-agent/app"
)

var versionCommand = flags.Command{
	Description: "Agent version.",
	Target: func(opts flags.Options) error {
		fmt.Printf("%s (commit: %s)\n", app.Version, app.Commit)
		return nil
	},
}
