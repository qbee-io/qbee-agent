package env

import "os"

// Get returns the value of the environment variable with the given name.
// If the environment variable is not set, the defaultValue is returned.
func Get(name, defaultValue string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}

	return defaultValue
}
