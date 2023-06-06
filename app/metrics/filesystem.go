package metrics

import (
	"math"
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
//
//	{
//	 "size": 486903968,
//	 "used": 62249152,
//	 "avail": 399848008,
//	 "use": 14,
//	 "inodes": 30990336,
//	 "iused": 844508,
//	 "ifree": 30145828,
//	 "iuse": 3
//	}
type FilesystemValues struct {
	Size      uint64  `json:"size"`
	Used      uint64  `json:"used"`
	Available uint64  `json:"avail"`
	Use       float64 `json:"use"`
	INodes    uint64  `json:"inodes"`
	IUsed     uint64  `json:"iused"`
	IFree     uint64  `json:"ifree"`
	IUse      uint64  `json:"iuse"`
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

		size := uint64(st.Blocks) * uint64(st.Bsize) / fsBlockSize
		free := uint64(st.Bavail) * uint64(st.Bsize) / fsBlockSize

		var iuse uint64
		var use float64

		if size > 0 {
			use = 100.0 - (float64(free)/float64(size))*100.0
		}

		if st.Files > 0 {
			iuse = 100 - uint64(st.Ffree*100/st.Files)
		}

		metrics[i] = Metric{
			Label:     Filesystem,
			Timestamp: time.Now().Unix(),
			ID:        mount,
			Values: Values{
				FilesystemValues: &FilesystemValues{
					Size:      size,
					Used:      size - free,
					Available: free,
					Use:       math.Round(use*100) / 100.0,
					INodes:    uint64(st.Files),
					IUsed:     uint64(st.Files - st.Ffree),
					IFree:     uint64(st.Ffree),
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
