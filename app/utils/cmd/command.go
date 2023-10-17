package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

const (
	helpOption = "help"
)

// Command represents a level in the command tree.
// If Target is set, it will be executed when reaching the Command.
// Otherwise, one of the SubCommands must be requested.
// If no sub-command is provided, show help for the current level.
type Command struct {
	// Description of the command.
	Description string

	// Options to be applied before Target or SubCommands are executed.
	Options []Option

	// SubCommands which can be used from the Command.
	SubCommands map[string]Command

	// Target function to be executed when the Command is called.
	Target func(opts Options) error
}

// Execute Target of the Command (if set), one of the sub-commands or show help.
func (cmd Command) Execute(args []string, opts Options) error {
	var err error
	if args, opts, err = cmd.evaluateArgs(args, opts); err != nil {
		cmd.renderHelp()
		return err
	}

	if _, helpRequested := opts[helpOption]; helpRequested {
		cmd.renderHelp()
		return nil
	}

	if cmd.Target != nil {
		return cmd.Target(opts)
	}

	if len(args) == 0 {
		cmd.renderHelp()
		return fmt.Errorf("command required")
	}

	subCommand, ok := cmd.SubCommands[args[0]]
	if !ok {
		cmd.renderHelp()
		return fmt.Errorf("unknown command")
	}

	return subCommand.Execute(args[1:], opts)
}

// renderOptions for the command.
func (cmd Command) renderOptions() {
	if len(cmd.Options) == 0 {
		return
	}

	fmt.Println("\nOptions:")

	writer := tabwriter.NewWriter(os.Stdout, 0, 1, 2, ' ', 0)
	for _, opt := range cmd.Options {
		if opt.Hidden {
			continue
		}

		line := "  "

		if opt.Short == "" {
			line += fmt.Sprintf("    ")
		} else {
			line += fmt.Sprintf("-%s, ", opt.Short)
		}

		line += fmt.Sprintf("--%s", opt.Name)

		if opt.Flag == "" {
			line += fmt.Sprintf(" %s", strings.ToUpper(strings.ReplaceAll(opt.Name, "-", "_")))
		}

		line += fmt.Sprintf("\t%s\t", opt.Help)

		if opt.Required {
			line += "[required]\t"
		} else {
			line += "[optional]\t"
		}

		if opt.Default != "" {
			line += fmt.Sprintf("(default: %s)\t", opt.Default)
		}

		_, _ = fmt.Fprintln(writer, line)
	}
	_ = writer.Flush()

	fmt.Println()
}

// renderSubCommands returns sub-commands available for the command.
func (cmd Command) renderSubCommands() {
	if len(cmd.SubCommands) == 0 {
		return
	}

	// sort sub-commands by name
	subCommands := make([]string, 0, len(cmd.SubCommands))
	for subCmdName := range cmd.SubCommands {
		subCommands = append(subCommands, subCmdName)
	}

	sort.Strings(subCommands)

	fmt.Println("\nCommands:")

	writer := tabwriter.NewWriter(os.Stdout, 0, 1, 1, ' ', 0)
	for _, subCmdName := range subCommands {
		_, _ = fmt.Fprintln(writer, fmt.Sprintf("  %s\t- %s\t", subCmdName, cmd.SubCommands[subCmdName].Description))
	}
	_ = writer.Flush()

	fmt.Println()
}

// renderHelp prints help message of the command to the stdout.
func (cmd Command) renderHelp() {
	fmt.Printf("Usage: %s [global options] <command> [options] [<command> [options] ...]\n", os.Args[0])

	cmd.renderOptions()

	cmd.renderSubCommands()
}

// evaluateArgs evaluates argument applicable to the current Command, set options and return unprocessed arguments.
func (cmd Command) evaluateArgs(args []string, opts Options) ([]string, Options, error) {
	if opts == nil {
		opts = make(Options)
	}

	commandOptions := make(map[string]Option)

	for i := range cmd.Options {
		opt := cmd.Options[i]
		commandOptions["--"+opt.Name] = opt

		if opt.Short != "" {
			commandOptions["-"+opt.Short] = opt
		}

		if opt.Default != "" {
			opts[opt.Name] = opt.Default
		}
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--help" || arg == "-h" {
			opts[helpOption] = "y"
			return args, opts, nil
		}

		if strings.HasPrefix(arg, "-") {
			opt, ok := commandOptions[arg]
			if !ok {
				return nil, nil, fmt.Errorf("unknown option: %s", arg)
			}

			if opt.Flag != "" {
				opts[opt.Name] = opt.Flag
			} else {
				i++
				if i == len(args) {
					return nil, nil, fmt.Errorf("value required for %s", arg)
				}

				opts[opt.Name] = args[i]
			}
		} else {
			args = args[i:]
			break
		}
	}

	// check for required options
	for _, opt := range cmd.Options {
		if _, isSet := opts[opt.Name]; opt.Required && !isSet {
			return nil, nil, fmt.Errorf("--%s is required", opt.Name)
		}
	}

	return args, opts, nil
}
