module github.com/qbee-io/qbee-agent

go 1.20

require (
	github.com/google/go-tpm v0.3.3
	github.com/google/go-tpm-tools v0.3.9
	google.golang.org/protobuf v1.28.0
)

require golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect

require qbee.io/platform/shared v0.0.0

replace qbee.io/platform/shared => ../../shared
