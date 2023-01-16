package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const Debian = "debian:qbee"

func NewDockerRunner(t *testing.T, image string) *DockerRunner {
	output, err := exec.Command("docker", "run", "--rm", "--detach", image, "sleep", "60").Output()
	if err != nil {
		panic(err)
	}

	container := strings.TrimSpace(string(output))

	runner := &DockerRunner{
		t:         t,
		container: container,
	}

	t.Cleanup(runner.Close)
	t.Parallel()

	return runner
}

type DockerRunner struct {
	t         *testing.T
	container string
}

func (runner *DockerRunner) Close() {
	_ = exec.Command("docker", "kill", runner.container).Run()
}

func (runner *DockerRunner) Exec(cmd ...string) ([]byte, error) {
	execCommand := append([]string{"exec", runner.container}, cmd...)

	output, err := exec.Command("docker", execCommand...).Output()

	return bytes.TrimSpace(output), err
}

func (runner *DockerRunner) MustExec(cmd ...string) []byte {
	output, err := runner.Exec(cmd...)
	if err != nil {
		if len(output) > 0 {
			fmt.Println("stdout:", string(output))
		}

		execExitErr := new(exec.ExitError)
		if errors.As(err, &execExitErr) && len(execExitErr.Stderr) > 0 {
			fmt.Println("stderr:", string(execExitErr.Stderr))
		}

		panic(err)
	}

	return output
}

func (runner *DockerRunner) CreateFile(path string, contents []byte) {
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

	if err := exec.Command("docker", "cp", fd.Name(), containerPath).Run(); err != nil {
		panic(err)
	}
}

func (runner *DockerRunner) ReadFile(path string) []byte {
	return runner.MustExec("cat", path)
}

func (runner *DockerRunner) CreateJSON(path string, doc any) {
	docBytes, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}

	runner.CreateFile(path, docBytes)
}
