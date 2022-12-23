package cmd

import (
	"fmt"
	"os"
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

	fmt.Println("\nCommands:")

	writer := tabwriter.NewWriter(os.Stdout, 0, 1, 1, ' ', 0)
	for subCmdName, subCmd := range cmd.SubCommands {
		_, _ = fmt.Fprintln(writer, fmt.Sprintf("  %s\t- %s\t", subCmdName, subCmd.Description))
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

type Option struct {
	// Name of the option argument. When set to "option", "--option <val>" arguments will be expected.
	Name string

	// Short option name. When set to "o", "-o <val>" arguments will be expected.
	Short string

	// Help message displayed to the user.
	Help string

	// Flag if set to non-empty string, will be used as value when command line option is provided.
	// It won't consume value argument.
	Flag string

	// Required option. If no value is set, help message will be displayed.
	Required bool

	// Default value used if options is not set.
	// If no value is set and Default is an empty string, Target won't be executed.
	Default string

	// Value of the Option after evaluating flags.
	Value string

	// Hidden if set, the option won't be returned in the help message.
	// This is useful for internal options.
	Hidden bool
}

// Options represent a mapping of Option.Name to Option.Value for options selected by a user.
type Options map[string]string
