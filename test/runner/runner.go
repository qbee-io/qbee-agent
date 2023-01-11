package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

const Debian = "debian:qbee"

func New(t *testing.T, image string) *Runner {
	output, err := exec.Command("docker", "run", "--rm", "--detach", image, "sleep", "60").Output()
	if err != nil {
		panic(err)
	}

	container := strings.TrimSpace(string(output))

	runner := &Runner{
		t:         t,
		start:     time.Now(),
		container: container,
	}

	t.Cleanup(runner.Close)
	t.Parallel()

	return runner
}

type Runner struct {
	t         *testing.T
	start     time.Time
	container string
}

func (runner *Runner) log(msg string, args ...any) {
	msg = fmt.Sprintf(msg, args...)

	_, file, line, ok := runtime.Caller(1)
	if ok {
		fmt.Printf("%s:%d: [%v] %s\n", file, line, time.Since(runner.start), msg)
	} else {
		fmt.Printf("[%v] %s\n", time.Since(runner.start), msg)
	}
}

func (runner *Runner) Close() {
	runner.log("closing")
	_ = exec.Command("docker", "kill", runner.container).Run()
	runner.log("closed")
}

func (runner *Runner) Exec(cmd ...string) ([]byte, error) {
	runner.log("executing %v", cmd)
	defer runner.log("done")

	execCommand := append([]string{"exec", runner.container}, cmd...)

	output, err := exec.Command("docker", execCommand...).Output()

	return bytes.TrimSpace(output), err
}

func (runner *Runner) MustExec(cmd ...string) []byte {
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

func (runner *Runner) CreateFile(path string, contents []byte) {
	runner.log("creating file %s", path)
	defer runner.log("done")

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

func (runner *Runner) ReadFile(path string) []byte {
	runner.log("reading file %s", path)
	defer runner.log("done")

	return runner.MustExec("cat", path)
}

func (runner *Runner) CreateJSON(path string, doc any) {
	runner.log("creating JSON %s", path)
	defer runner.log("done")

	docBytes, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}

	runner.CreateFile(path, docBytes)
}
