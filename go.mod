module github.com/qbee-io/qbee-agent

go 1.20

require (
	github.com/google/go-tpm v0.3.3
	github.com/google/go-tpm-tools v0.3.9
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/google/uuid v1.3.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
)

require qbee.io/platform v0.0.0

replace qbee.io/platform => ../../src
