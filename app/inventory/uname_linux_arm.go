//go:build linux && arm

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

	systemInfo.Host = uint8SliceToString(utsname.Nodename[:])
	systemInfo.UQHost = uint8SliceToString(utsname.Nodename[:])
	systemInfo.FQHost = uint8SliceToString(utsname.Nodename[:])
	systemInfo.Release = uint8SliceToString(utsname.Release[:])
	systemInfo.Version = uint8SliceToString(utsname.Version[:])
	systemInfo.Architecture = uint8SliceToString(utsname.Machine[:])

	domainName := uint8SliceToString(utsname.Domainname[:])
	if domainName != "" && domainName != "(none)" {
		systemInfo.FQHost = fmt.Sprintf("%s.%s", systemInfo.UQHost, domainName)
	}

	return nil
}

// uint8SliceToString converts slice []uint8 into a string.
func uint8SliceToString(val []uint8) string {
	buf := make([]byte, 0, len(val))
	for _, b := range val {
		if b == 0 {
			break
		}

		buf = append(buf, byte(b))
	}

	return string(buf[:])
}
