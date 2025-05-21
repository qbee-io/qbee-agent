module go.qbee.io/agent

go 1.23.0

toolchain go1.23.8

require (
	github.com/UserExistsError/conpty v0.1.4
	github.com/creack/pty v1.1.21
	github.com/google/go-tpm v0.9.0
	github.com/google/go-tpm-tools v0.4.2
	github.com/shirou/gopsutil/v4 v4.25.4
	github.com/xtaci/smux v1.5.34
	go.qbee.io/transport v1.25.12
	golang.org/x/sys v0.33.0
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-sev-guest v0.9.3 // indirect
	github.com/google/go-tdx-guest v0.2.3-0.20231011100059-4cf02bed9d33 // indirect
	github.com/google/logger v1.1.1 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/crypto v0.37.0 // indirect
)

//replace go.qbee.io/transport => ../transport
