//go:build linux && !arm

package inventory

import (
	"fmt"
	"syscall"
)

// parseUnameSyscall fills out system info based on results from uname syscall.
func (systemInfo *SystemInfo) parseUnameSyscall() error {
	utsname := new(syscall.Utsname)

	if err := syscall.Uname(utsname); err != nil {
		return fmt.Errorf("error calling Uname syscall: %w", err)
	}

	systemInfo.Host = int8SliceToString(utsname.Nodename[:])
	systemInfo.UQHost = int8SliceToString(utsname.Nodename[:])
	systemInfo.FQHost = int8SliceToString(utsname.Nodename[:])
	systemInfo.Release = int8SliceToString(utsname.Release[:])
	systemInfo.Version = int8SliceToString(utsname.Version[:])
	systemInfo.Architecture = int8SliceToString(utsname.Machine[:])

	domainName := int8SliceToString(utsname.Domainname[:])
	if domainName != "" && domainName != "(none)" {
		systemInfo.FQHost = fmt.Sprintf("%s.%s", systemInfo.UQHost, domainName)
	}

	return nil
}

// int8SliceToString converts slice []int8 into a string.
func int8SliceToString(val []int8) string {
	buf := make([]byte, 0, len(val))
	for _, b := range val {
		if b == 0 {
			break
		}

		buf = append(buf, byte(b))
	}

	return string(buf[:])
}
