package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Debian is the image used by the runner.
const Debian = "debian:qbee"

// New creates a new runner for the given test.
func New(t *testing.T) *Runner {
	cmdArgs := []string{
		"run",
		"--rm",                                            // remove container after container exits
		"-v", "/var/run/docker.sock:/var/run/docker.sock", // mount docker socket
		"-v", "/sys/fs/cgroup:/sys/fs/cgroup:ro", // mount cgroup for docker
		"--tmpfs", "/tmp",
		"--tmpfs", "/run",
		"--tmpfs", "/run/lock",
		"--cap-add=NET_ADMIN", // allow control of firewall
		"--detach",            // launch in background
		Debian,
		"sleep", "600", // force exit container after 10 minutes
	}

	output, err := exec.Command("docker", cmdArgs...).Output()
	if err != nil {
		panic(err)
	}

	container := strings.TrimSpace(string(output))

	runner := &Runner{
		t:         t,
		container: container,
	}

	t.Cleanup(runner.Close)

	return runner
}

// Runner provides a convenient way to run the agent in a container.
type Runner struct {
	t         *testing.T
	container string
}

// Close kills the container.
func (runner *Runner) Close() {
	_ = exec.Command("docker", "kill", runner.container).Run()
}

// Pause processes within the runner container.
func (runner *Runner) Pause() {
	_ = exec.Command("docker", "pause", runner.container).Run()
}

// Unpause processes within the runner container.
func (runner *Runner) Unpause() {
	_ = exec.Command("docker", "unpause", runner.container).Run()
}

// Exec executes the given command in the runner container.
// It returns the output of the command and an error if the command failed.
func (runner *Runner) Exec(cmd ...string) ([]byte, error) {
	execCommand := append([]string{"exec", runner.container}, cmd...)

	execCmd := exec.Command("docker", execCommand...)
	stderr := new(bytes.Buffer)
	execCmd.Stderr = stderr
	output, err := execCmd.Output()

	for _, line := range strings.Split(stderr.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			fmt.Println(line)
		}
	}

	return bytes.TrimSpace(output), err
}

// MustExec executes the given command and panics if it fails.
func (runner *Runner) MustExec(cmd ...string) []byte {
	output, err := runner.Exec(cmd...)
	if err != nil {
		if len(output) > 0 {
			fmt.Println("stdout:", string(output))
		}

		panic(err)
	}

	return output
}

// CreateFile creates a file with the given path and contents.
func (runner *Runner) CreateFile(path string, contents []byte) {
	fd, err := os.CreateTemp(runner.t.TempDir(), "qbee-test-*")
	if err != nil {
		panic(err)
	}

	defer os.Remove(fd.Name())

	_, err = fd.Write(contents)

	fd.Close()

	if err != nil {
		panic(err)
	}

	containerPath := fmt.Sprintf("%s:%s", runner.container, path)

	if err = exec.Command("docker", "cp", fd.Name(), containerPath).Run(); err != nil {
		panic(err)
	}
}

// ReadFile reads the contents of the given file from the runner.
func (runner *Runner) ReadFile(path string) []byte {
	fd, err := os.CreateTemp(runner.t.TempDir(), "qbee-test-*")
	if err != nil {
		panic(err)
	}
	_ = fd.Close()

	defer os.Remove(fd.Name())

	containerPath := fmt.Sprintf("%s:%s", runner.container, path)

	if err = exec.Command("docker", "cp", containerPath, fd.Name()).Run(); err != nil {
		panic(err)
	}

	var output []byte
	if output, err = os.ReadFile(fd.Name()); err != nil {
		panic(err)
	}

	return output
}

// CreateJSON creates a file with the given path and JSON-encodes the given document.
func (runner *Runner) CreateJSON(path string, doc any) {
	docBytes, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}

	runner.CreateFile(path, docBytes)
}
