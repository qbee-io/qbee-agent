package linux

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
)

type ProcessStatus struct {
	Owner  string
	Memory uint64
}

// GetProcessStatus returns ProcessStatus based on /proc/*/status.
// See `man proc` -> `/proc/[pid]/status section for details on the file format.
func GetProcessStatus(pid string) (*ProcessStatus, error) {
	statusFilePath := path.Join(ProcFS, pid, "status")

	fp, err := os.Open(statusFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", statusFilePath, err)
	}

	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	processStatus := new(ProcessStatus)

	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading line from %s: %w", statusFilePath, err)
		}

		fields := strings.Fields(scanner.Text())

		switch fields[0] {
		case "Uid:":
			effectiveUID := fields[3]

			var userInfo *user.User

			if userInfo, err = user.LookupId(effectiveUID); err != nil {
				// if user lookup fails, use UID for Username
				userInfo = &user.User{Username: effectiveUID}
			}

			processStatus.Owner = userInfo.Username
		case "RssAnon:", "RssFile:", "RssShmem:":
			if len(fields) != 3 || fields[2] != "kB" {
				return nil, fmt.Errorf("unsupported file format: %s", statusFilePath)
			}

			var value uint64
			if value, err = strconv.ParseUint(fields[1], 10, 64); err != nil {
				return nil, fmt.Errorf("error parsing value for %s in %s: %w", fields[0], statusFilePath, err)
			}

			processStatus.Memory += value
		}
	}

	return processStatus, nil
}
