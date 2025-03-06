module go.qbee.io/agent

go 1.22

require (
	github.com/UserExistsError/conpty v0.1.4
	github.com/creack/pty v1.1.21
	github.com/google/go-tpm v0.9.0
	github.com/google/go-tpm-tools v0.4.2
	github.com/shirou/gopsutil/v4 v4.24.6
	github.com/xtaci/smux v1.5.24
	go.qbee.io/transport v1.24.33
	golang.org/x/sys v0.28.0
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-sev-guest v0.9.3 // indirect
	github.com/google/go-tdx-guest v0.2.3-0.20231011100059-4cf02bed9d33 // indirect
	github.com/google/logger v1.1.1 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/crypto v0.31.0 // indirect
)

//replace go.qbee.io/transport => ../transport
