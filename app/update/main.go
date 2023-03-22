package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

func main() {
	agentBin, err := os.ReadFile("/home/constructor/src/platform/apps/agent-v2/bin/qbee-agent")
	if err != nil {
		panic(err)
	}

	x, err := New(agentBin)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	cmd := x.CommandContext(ctx, "-h")

	out, err := cmd.Output()
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))
}

// Exec is an in-memory executable code unit.
type Exec struct {
	f     *os.File
	opts  []func(cmd *exec.Cmd)
	clean func() error
}

func New(b []byte) (*Exec, error) {
	f, err := open(b)
	if err != nil {
		return nil, err
	}
	e := &Exec{f: f}
	return e, nil
}

func (m *Exec) CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	exe := exec.CommandContext(ctx, m.f.Name(), args...)
	return exe
}

func (m *Exec) Close() error {
	if err := clean(m.f); err != nil {
		if m.clean != nil {
			_ = m.clean()
		}
		return err
	}
	if m.clean == nil {
		return nil
	}
	return m.clean()
}

func open(b []byte) (*os.File, error) {
	fd, err := unix.MemfdCreate("", unix.MFD_CLOEXEC)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), fd))
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func clean(f *os.File) error {
	return f.Close()
}
