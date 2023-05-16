VERSION=0000.00
VERSION_VAR=github.com/qbee-io/qbee-agent/app.Version
COMMIT_VAR=github.com/qbee-io/qbee-agent/app.Commit
COMMIT=$(shell git describe --always --dirty --abbrev=0)
GOOS=linux
GOARCH=amd64

# PUBLIC_SIGNING_KEY is a public part of services/devicehub/cmd/agent-upload/dev-signing.key
# For production release, it must be replaced with the correct public key.
# Use cmd/get-public-signing-key/main.go to obtain a public key from a private key.
PUBLIC_SIGNING_KEY=xSHbUBG7LTuNfXd3zod4EX8_Es8FTCINgrjvx1WXFE4.plCHzlDAeb3IWW1wK6P6paMRYO4f8qceV3lrNCqNpWo
PUBLIC_SINGING_KEY_VAR=github.com/qbee-io/qbee-agent/app/binary.PublicSigningKey

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION) -X $(PUBLIC_SINGING_KEY_VAR)=$(PUBLIC_SIGNING_KEY) -X $(COMMIT_VAR)=$(COMMIT)" \
		-o bin/qbee-agent cmd/agent/main.go

auto-build:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build; done

docker-image:
	docker build -t debian:qbee .

auto-docker-image:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build docker-image; done

test:
	go test ./app/...
