package cmd

import "strings"

// Options represent a mapping of Option.Name to Option.Value for options selected by a user.
type Options map[string]string

// RemainingArgs returns the remaining positional arguments stored in Options.
func (o Options) RemainingArgs() []string {
	v, ok := o[RemainingArgsKey]
	if !ok || v == "" {
		return nil
	}

	return strings.Split(v, "\x00")
}

// MultiValues returns all values accumulated for a multi-value option.
func (o Options) MultiValues(key string) []string {
	v, ok := o[key]
	if !ok || v == "" {
		return nil
	}

	return strings.Split(v, "\x00")
}

// Option represents a command line option.
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

	// Multi if set, the option can be specified multiple times.
	// All values are stored as null-byte separated string and can be retrieved with Options.MultiValues.
	Multi bool
}
