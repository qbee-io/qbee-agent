//go:build linux

package inventory

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
	"github.com/qbee-io/qbee-agent/app/log"
)

// CollectPortsInventory returns populated Ports inventory based on current system status.
func CollectPortsInventory() (*Ports, error) {
	ports := new(Ports)

	var protocols = []string{"tcp", "tcp6", "udp", "udp6"}

	inodesMap, err := loadProcessFDInodes()
	if err != nil {
		return nil, err
	}

	for _, protocol := range protocols {
		var listeningPorts []Port
		if listeningPorts, err = parseNetworkPorts(protocol, inodesMap); err != nil {
			return nil, err
		}

		if len(listeningPorts) > 0 {
			ports.Ports = append(ports.Ports, listeningPorts...)
		}
	}

	return ports, nil
}

// loadProcessFDInodes loads mapping of processes' open files inodes to file paths.
func loadProcessFDInodes() (map[uint64]string, error) {
	// scan all open files for all running processes
	runningProcesses, err := linux.ListRunningProcesses()
	if err != nil {
		return nil, fmt.Errorf("error listing /proc/<pid> directories: %w", err)
	}

	result := make(map[uint64]string)

	var fdPaths []string
	var fileStat os.FileInfo
	var fdDir *os.File

	for _, pid := range runningProcesses {
		processProcPath := path.Join(linux.ProcFS, pid)
		fdDirPath := path.Join(processProcPath, "fd")

		if fdDir, err = os.Open(fdDirPath); err != nil {
			return nil, fmt.Errorf("error openning %s: %w", fdDirPath, err)
		}

		fdPaths, err = fdDir.Readdirnames(-1)

		_ = fdDir.Close()

		if err != nil {
			return nil, fmt.Errorf("error listing files in %s: %w", fdDirPath, err)
		}

		for _, fdPath := range fdPaths {
			// get file info for each open file
			if fileStat, err = os.Stat(fdPath); err != nil {
				log.Debugf("cannot get stats of %s: %v", fdPath, err)
				continue
			}

			fileStatT := fileStat.Sys().(*syscall.Stat_t)

			result[fileStatT.Ino] = processProcPath
		}
	}

	return result, nil
}

// parseNetworkPorts parses /proc/net/<protocol> file format and returns a list of listening ports.
func parseNetworkPorts(protocol string, inodesMap map[uint64]string) ([]Port, error) {
	procFilePath := path.Join("/proc/net", protocol)

	file, err := os.Open(procFilePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("error opening %s: %w", procFilePath, err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	ports := make([]Port, 0)

	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading %s: %w", procFilePath, err)
		}

		fields := strings.Fields(scanner.Text())

		// Skip non-listening sockets (that also skips the header).
		// Listening socket is the one which remote address port is zero.
		if remoteAddress := fields[2]; !strings.HasSuffix(remoteAddress, ":0000") {
			continue
		}

		localAddress := strings.Split(fields[1], ":")
		inode := fields[9]

		address := parserKernelNetworkAddress(localAddress[0])
		port, _ := strconv.ParseInt(localAddress[1], 16, 0)

		var inodeInt int
		if inodeInt, err = strconv.Atoi(inode); err != nil {
			return nil, fmt.Errorf("error parsing inode %s: %w", inode, err)
		}

		var cmdLine []byte

		// lookup socket's inode in the inode map to identify the process owning it
		if fileDescriptorPath, found := inodesMap[uint64(inodeInt)]; found {
			processID := strings.SplitN(fileDescriptorPath, "/", 4)[2]
			cmdLinePath := fmt.Sprintf("/proc/%s/cmdline", processID)
			if cmdLine, err = os.ReadFile(cmdLinePath); err != nil {
				return nil, fmt.Errorf("error reading %s: %w", cmdLinePath, err)
			}
		}

		ports = append(ports, Port{
			Protocol: protocol,
			Socket:   fmt.Sprintf("%s:%d", address, port),
			Process:  strings.TrimSpace(strings.ReplaceAll(string(cmdLine), "\000", " ")),
		})
	}

	return ports, nil
}

// parserKernelNetworkAddress takes kernel encoding of a network address and returns a human-readable form.
// 0100007F -> 127.0.0.1
// 0000000000000000FFFF00000100007F -> 127.0.0.1
// 00000000000000000000000001000000 -> ::1
func parserKernelNetworkAddress(addr string) net.IP {
	a, _ := hex.DecodeString(addr)

	var addrBytes []byte

	// change endianness
	switch len(a) {
	case net.IPv4len:
		addrBytes = []byte{a[3], a[2], a[1], a[0]}
	case net.IPv6len:
		addrBytes = []byte{
			a[3], a[2], a[1], a[0],
			a[7], a[6], a[5], a[4],
			a[11], a[10], a[9], a[8],
			a[15], a[14], a[13], a[12],
		}

	}

	return addrBytes
}
