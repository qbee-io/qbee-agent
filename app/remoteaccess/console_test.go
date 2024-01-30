package remoteaccess

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xtaci/smux"
	"go.qbee.io/transport"

	"go.qbee.io/agent/app/utils/assert"
)

func Test_Console_StreamClose(t *testing.T) {
	// this tests that the console session is closed when the stream is closed
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cli, devCli, _ := transport.NewEdgeMock(t)

	service := New()
	devCli.WithHandler(transport.MessageTypePTY, service.HandleConsole)

	initCmd, err := json.Marshal(&transport.PTYCommand{Type: transport.PTYCommandTypeResize, Cols: 10, Rows: 10})
	assert.NoError(t, err)

	var stream *smux.Stream
	stream, err = cli.OpenStream(ctx, transport.MessageTypePTY, initCmd)
	assert.NoError(t, err)

	assert.Length(t, service.consoleMap, 1)

	assert.NoError(t, stream.Close())

	assert.EventuallyTrue(t, func() bool { return len(service.consoleMap) == 0 }, time.Second)
}

func Test_Console_CommandClose(t *testing.T) {
	// this tests that the console session is closed when the shell command is closed
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cli, devCli, _ := transport.NewEdgeMock(t)

	service := New()
	devCli.WithHandler(transport.MessageTypePTY, service.HandleConsole)

	initCmd, err := json.Marshal(&transport.PTYCommand{Type: transport.PTYCommandTypeResize, Cols: 10, Rows: 10})
	assert.NoError(t, err)

	var stream *smux.Stream
	stream, err = cli.OpenStream(ctx, transport.MessageTypePTY, initCmd)
	assert.NoError(t, err)

	assert.Length(t, service.consoleMap, 1)

	_, err = stream.Write([]byte("exit\n"))
	assert.NoError(t, err)

	assert.EventuallyTrue(t, func() bool { return len(service.consoleMap) == 0 }, time.Second)
}
