package agent

import (
	"testing"

	"github.com/qbee-io/qbee-agent/app/test"
)

func Test_Update_Manual(t *testing.T) {
	r := test.New(t)
	r.Bootstrap()

	version := r.MustExec("qbee-agent", "version")

	test.Equal(t, string(version), "0000.00")

	r.MustExec("qbee-agent", "-l", "DEBUG", "update")

	version = r.MustExec("qbee-agent", "version")

	test.Equal(t, string(version), "2023.01")
}

func Test_Update_Manual_UsingRelativePath(t *testing.T) {
	r := test.New(t)
	r.Bootstrap()

	test.Equal(t, string(r.MustExec("pwd")), "/app")
	r.MustExec("cp", "/usr/sbin/qbee-agent", "/app/qbee-agent")
	r.MustExec("./qbee-agent", "-l", "DEBUG", "update")

	// make sure the original binary is not overwritten
	version := r.MustExec("qbee-agent", "version")
	test.Equal(t, string(version), "0000.00")

	// make sure the binary we used to update is updated
	version = r.MustExec("/app/qbee-agent", "version")
	test.Equal(t, string(version), "2023.01")
}

func Test_Update_Automatic(t *testing.T) {
	r := test.New(t)
	r.Bootstrap()

	version := r.MustExec("qbee-agent", "version")

	test.Equal(t, string(version), "0000.00")

	// Agent always starts with initial run and that will trigger the update.
	// When th update is successful, the agent will terminate itself.
	r.MustExec("qbee-agent", "start")

	version = r.MustExec("qbee-agent", "version")

	test.Equal(t, string(version), "2023.01")
}
