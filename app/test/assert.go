package test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Equal asserts that a and b are equal.
func Equal(t *testing.T, got, expected interface{}) {
	if !reflect.DeepEqual(got, expected) {
		failTest(t, "expected:\n%#v\ngot:\n%#v", expected, got)
	}
}

// HasPrefix asserts that value starts with prefix.
func HasPrefix(t *testing.T, value, prefix string) {
	if !strings.HasPrefix(value, prefix) {
		failTest(t, "missing prefix %s in %s", prefix, value)
	}
}

// Empty asserts that provided value is empty.
func Empty(t *testing.T, value interface{}) {
	if !reflect.ValueOf(value).IsZero() {
		failTest(t, "expected empty value, got %v", value)
	}
}

// Length asserts length of value (slice, string, etc.).
func Length(t *testing.T, value any, expectedLength int) {
	gotLength := reflect.ValueOf(value).Len()
	if gotLength != expectedLength {
		failTest(t, "expected length %d, got %d", expectedLength, gotLength)
	}
}

// MatchString asserts that provided value matches the (regex) pattern.
func MatchString(t *testing.T, value, pattern string) {
	if match, err := regexp.MatchString(pattern, value); err != nil {
		failTest(t, "regexp error: %v", err)
	} else if !match {
		failTest(t, "%s doesn't match pattern %s", value, pattern)
	}
}

// False asserts that provided value is false.
func False(t *testing.T, value bool) {
	if value {
		failTest(t, "expected false, got true")
	}
}

// failTest prints out a formatted failure message and fails the test immediately.
func failTest(t *testing.T, msg string, args ...any) {
	logMsg := fmt.Sprintf(msg, args...)

	_, file, line, ok := runtime.Caller(2)

	prefix := "    "
	if ok {
		prefix = fmt.Sprintf("%s%s:%d: ", prefix, filepath.Base(file), line)
	}

	lines := strings.Split(logMsg, "\n")

	for i, line := range lines {
		fmt.Printf("%s%s\n", prefix, line)
		if i == 0 {
			prefix = strings.Repeat(" ", len(prefix))
		}
	}

	t.FailNow()
}
