//go:build linux

package inventory

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
)

// CollectProcessesInventory returns populated Processes inventory based on current system status.
// Based on https://www.kernel.org/doc/html/latest/filesystems/proc.html#id10
func CollectProcessesInventory() (*Processes, error) {
	runningProcesses, err := linux.ListRunningProcesses()
	if err != nil {
		return nil, fmt.Errorf("error listing running processes: %w", err)
	}

	var totalJiffiesT0, totalJiffiesT1 uint64
	var processJiffiesT0, processJiffiesT1 map[string]linux.ProcessStats

	firstReadTime := time.Now()

	// collect total CPU jiffies
	if totalJiffiesT0, err = getTotalJiffies(); err != nil {
		return nil, err
	}

	// collect CPU stats for each process
	if processJiffiesT0, err = getProcessStats(runningProcesses); err != nil {
		return nil, err
	}

	// Because CPU utilization can be calculated only over time,
	// we need to wait a bit and collect another set of stats.
	// The wait time is adjusted by how long it took to process the first batch of jiffies.
	time.Sleep(time.Second - time.Since(firstReadTime))

	// collect total CPU jiffies (again)
	if totalJiffiesT1, err = getTotalJiffies(); err != nil {
		return nil, err
	}

	// collect CPU stats for each process (again)
	if processJiffiesT1, err = getProcessStats(runningProcesses); err != nil {
		return nil, err
	}

	// get system memory information
	var memInfo *linux.MemInfo
	if memInfo, err = linux.GetMemInfo(); err != nil {
		return nil, err
	}

	processes := &Processes{
		Processes: make([]Process, 0, len(processJiffiesT1)),
	}

	totalJiffiesPerCPU := (totalJiffiesT1 - totalJiffiesT0) / uint64(runtime.NumCPU())

	var jiffiesT0, jiffiesT1 uint64
	var processStatus *linux.ProcessStatus

	for pid, processStats := range processJiffiesT1 {
		// calculate CPU utilization during wait time
		if jiffiesT1, err = processStats.GetJiffies(); err != nil {
			return nil, err
		}

		// it's possible that process was started during our measurement window, so need to account for that
		psT0, ok := processJiffiesT0[pid]
		if ok {
			if jiffiesT0, err = psT0.GetJiffies(); err != nil {
				return nil, err
			}
		}

		// calculate utilization delta
		jiffiesDelta := jiffiesT1 - jiffiesT0

		// get process status
		if processStatus, err = linux.GetProcessStatus(pid); err != nil {
			return nil, err
		}

		process := Process{
			PID:     processStats.PID(),
			User:    processStatus.Owner,
			Memory:  float64(processStatus.Memory*10000/memInfo.TotalMemory) / 100.0,
			CPU:     float64(jiffiesDelta*10000/totalJiffiesPerCPU) / 100.0,
			Command: processStats.Command(),
		}

		processes.Processes = append(processes.Processes, process)
	}

	return processes, nil
}

const procStatBufferSize = 1024

// getProcessStats returns a map of PID -> process stats for currently running processes.
func getProcessStats(runningProcesses []string) (map[string]linux.ProcessStats, error) {
	processStats := make(map[string]linux.ProcessStats)

	var n int
	var err error
	var procStatFile *os.File
	var statFilePath string

	buf := make([]byte, procStatBufferSize)

	for _, pid := range runningProcesses {
		statFilePath = path.Join(linux.ProcFS, pid, "stat")

		if procStatFile, err = os.Open(statFilePath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return nil, fmt.Errorf("error opening %s: %w", statFilePath, err)
		}

		// read file contents to a buffer
		n, err = procStatFile.Read(buf)

		// close the file
		_ = procStatFile.Close()

		// check for errors
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", statFilePath, err)
		}

		if processStats[pid], err = linux.NewProcessStats(string(buf[0:n])); err != nil {
			return nil, err
		}
	}

	return processStats, nil
}

// getTotalJiffies returns a sum of all jiffies from /proc/stat
// We could use C.sysconf(C._SC_CLK_TCK), but that would require CGO and that's not helping with portability.
func getTotalJiffies() (uint64, error) {
	filePath := path.Join(linux.ProcFS, "stat")

	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	defer file.Close()

	// we don't need to read the whole file, we only care about the first line
	buf := make([]byte, 512)
	if _, err = file.Read(buf); err != nil {
		return 0, fmt.Errorf("error reading contents of %s: %w", filePath, err)
	}

	firstLine := string(buf[0:bytes.Index(buf, []byte("\n"))])
	fields := strings.Fields(firstLine)

	var total, value uint64
	for i := 1; i < len(fields); i++ {
		value, err = strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing contents of %s: %w", filePath, err)
		}

		total += value
	}

	return total, nil
}
