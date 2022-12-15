package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/log"
)

const failureExitCode = 1

const mainHelp = `
Usage: %s <command>

Commands:
	help - display this message
	bootstrap - bootstrap device
	start - start the agent
`

func main() {
	if len(os.Args) == 1 {
		printHelp()
	}

	var err error

	switch os.Args[1] {
	case "bootstrap":
		err = bootstrap(os.Args[2:])
	case "start":
		err = start(os.Args[2:])
	case "help":
		printHelp()
	}

	if err != nil {
		log.Errorf(err.Error())
		os.Exit(failureExitCode)
	}
}

// start the control process which will run the agent.
func start(args []string) error {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	configDir := flagSet.String("c", agent.DefaultConfigDir, "Config directory.")
	disableAutoUpdate := flagSet.Bool("u", false, "Disable auto-update.")

	if err := flagSet.Parse(args); err != nil {
		return fmt.Errorf("error parsing flags: %w", err)
	}

	ctx := context.Background()

	cfg, err := agent.LoadConfig(*configDir)
	if err != nil {
		return err
	}

	if *disableAutoUpdate || !cfg.AutoUpdate {
		return agent.Start(ctx, cfg)
	}

	return agent.StartWithAutoUpdate(ctx, cfg)
}

// boostrap device.
func bootstrap(args []string) error {
	var bootstrapKey string
	cfg := new(agent.Config)

	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	flagSet.StringVar(&cfg.Directory, "c", agent.DefaultConfigDir,
		"Config directory.")
	flagSet.StringVar(&bootstrapKey, "k", "",
		"Set the bootstrap key found in the user profile (required)")
	flagSet.StringVar(&cfg.DeviceHubServer, "s", agent.DefaultDeviceHubServer,
		"Set the server to bootstrap to. Don't set this if you are using www.app.qbee.io")
	flagSet.StringVar(&cfg.DeviceHubPort, "p", agent.DefaultDeviceHubPort,
		"Set the bootstrap key found in the user profile (required)")
	flagSet.StringVar(&cfg.TPMDevice, "t", "",
		"TPM device to use (e.g. /dev/tpm0)")
	flagSet.StringVar(&cfg.ProxyServer, "x", "",
		"Specify a proxy host to use")
	flagSet.StringVar(&cfg.ProxyPort, "X", "",
		"Specify a proxy port to use")
	flagSet.StringVar(&cfg.ProxyUser, "U", "",
		"Specify a proxy username")
	flagSet.StringVar(&cfg.ProxyPassword, "P", "",
		"Specify a proxy password")
	flagSet.BoolVar(&cfg.AutoUpdate, "u", true,
		"Enable auto-update. Use '-u=false' to disable.")

	if err := flagSet.Parse(args); err != nil {
		return fmt.Errorf("error parsing flags: %w", err)
	}

	bootstrapKey = strings.TrimSpace(bootstrapKey)
	if bootstrapKey == "" {
		return fmt.Errorf("bootstrap key (-k) is required")
	}

	ctx := context.Background()

	if err := agent.Bootstrap(ctx, cfg, bootstrapKey); err != nil {
		return fmt.Errorf("bootstrap error: %w", err)
	}

	return nil
}

func printHelp() {
	fmt.Printf(mainHelp, os.Args[0])
	os.Exit(failureExitCode)
}
