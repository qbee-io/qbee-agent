package metrics

import (
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// FilesystemValues
//
// Example payload:
// {
//  "size": 486903968,
//  "used": 62249152,
//  "avail": 399848008,
//  "use": 14,
//  "inodes": 30990336,
//  "iused": 844508,
//  "ifree": 30145828,
//  "iuse": 3
// }
type FilesystemValues struct {
	Size      int `json:"size"`
	Used      int `json:"used"`
	Available int `json:"avail"`
	Use       int `json:"use"`
	INodes    int `json:"inodes"`
	IUsed     int `json:"iused"`
	IFree     int `json:"ifree"`
	IUse      int `json:"iuse"`
}

const fsBlockSize = 1024

// CollectFilesystem returns filesystem metric for each filesystem mounted in read-write mode.
func CollectFilesystem() ([]Metric, error) {
	mounts, err := getFilesystemMounts()
	if err != nil {
		return nil, err
	}

	metrics := make([]Metric, len(mounts))

	for i, mount := range mounts {
		var st syscall.Statfs_t

		if err = syscall.Statfs(mount, &st); err != nil {
			return nil, err
		}

		size := int(st.Blocks) * int(st.Bsize) / fsBlockSize
		free := int(st.Bfree) * int(st.Bsize) / fsBlockSize

		var use, iuse int

		if size > 0 {
			use = 100 - (free * 100 / size)
		}

		if st.Files > 0 {
			iuse = 100 - int(st.Ffree*100/st.Files)
		}

		metrics[i] = Metric{
			Label:     Filesystem,
			Timestamp: time.Now().Unix(),
			ID:        mount,
			Values: Values{
				FilesystemValues: &FilesystemValues{
					Size:      size,
					Used:      size - free,
					Available: int(st.Bavail) * int(st.Bsize) / fsBlockSize,
					Use:       use,
					INodes:    int(st.Files),
					IUsed:     int(st.Files - st.Ffree),
					IFree:     int(st.Ffree),
					IUse:      iuse,
				},
			},
		}
	}

	return metrics, nil
}

// getFilesystemMounts returns a list of block-device mount points.
func getFilesystemMounts() ([]string, error) {
	supportedFilesystems, err := getSupportedFilesystems()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(linux.ProcFS, "mounts")

	mounts := make([]string, 0)

	err = utils.ForLinesInFile(path, func(line string) error {
		fields := strings.Fields(line)

		if fields[3] != "rw" && !strings.HasPrefix(fields[3], "rw,") {
			return nil
		}

		if supportedFilesystems[fields[2]] {
			mounts = append(mounts, fields[1])
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return mounts, nil
}

// getSupportedFilesystems returns a map of supported block-device filesystems.
func getSupportedFilesystems() (map[string]bool, error) {
	path := filepath.Join(linux.ProcFS, "filesystems")

	filesystems := make(map[string]bool)

	err := utils.ForLinesInFile(path, func(line string) error {
		if strings.HasPrefix(line, "nodev") {
			return nil
		}

		filesystems[strings.TrimSpace(line)] = true

		return nil
	})
	if err != nil {
		return nil, err
	}

	return filesystems, nil
}
