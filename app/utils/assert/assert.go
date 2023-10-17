package assert

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Equal asserts that a and b are equal.
func Equal(t *testing.T, got, expected interface{}) {
	if !reflect.DeepEqual(got, expected) {
		failTest(t, "expected:\n%#v\ngot:\n%#v", expected, got)
	}
}

// NotEqual asserts that a and b are not equal.
func NotEqual(t *testing.T, got, expected interface{}) {
	if reflect.DeepEqual(got, expected) {
		failTest(t, "expected not equal, got both with the same value:\n%#v", got)
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
	if value == nil {
		return
	}

	if !reflect.ValueOf(value).IsZero() {
		failTest(t, "expected empty value, got %v", value)
	}
}

// NotEmpty asserts that provided value is not empty.
func NotEmpty(t *testing.T, value interface{}) {
	if reflect.ValueOf(value).IsZero() {
		failTest(t, "expected non-empty value, got %v", value)
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

// recentDuration defines a duration which allows to check for recent timestamps.
const recentDuration = 20 * time.Second

// RecentUnix asserts that provided unix timestamp is recent (not older than recentDuration).
// It also asserts that provided timestamp is not in the future.
func RecentUnix(t *testing.T, timestamp int64) {
	providedTime := time.Unix(timestamp, 0)

	if time.Since(providedTime) > recentDuration {
		failTest(t, "expected recent timestamp, got %v -> %s", timestamp, providedTime)
	}

	if providedTime.After(time.Now()) {
		failTest(t, "expected timestamp in the past, got %v -> %s", timestamp, providedTime)
	}
}

// RecentUnixNano asserts that provided unix timestamp (in nanoseconds) is recent (not older than recentDuration).
// It also asserts that provided timestamp is not in the future.
func RecentUnixNano(t *testing.T, timestamp int64) {
	providedTime := time.Unix(0, timestamp)

	if time.Since(providedTime) > recentDuration {
		failTest(t, "expected recent timestamp, got %v -> %s", timestamp, providedTime)
	}

	if providedTime.After(time.Now()) {
		failTest(t, "expected timestamp in the past, got %v -> %s", timestamp, providedTime)
	}
}

var uuidRE = regexp.MustCompile(`^[a-f0-9]{8}-([a-f0-9]{4}-){3}[a-f0-9]{12}$`)

// UUID asserts that provided value is a valid UUID.
func UUID(t *testing.T, value string) {
	if !uuidRE.MatchString(value) {
		failTest(t, "%s is not a valid UUID", value)
	}
}

// False asserts that provided value is false.
func False(t *testing.T, value bool) {
	if value {
		failTest(t, "expected false, got true")
	}
}

// True asserts that provided value is true.
func True(t *testing.T, value bool) {
	if !value {
		failTest(t, "expected true, got false")
	}
}

// NoError asserts that provided error is nil.
func NoError(t *testing.T, err error) {
	if err != nil {
		failTest(t, "unexpected error: %v", err)
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
