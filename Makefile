VERSION=0000.00
VERSION_VAR=go.qbee.io/agent/app.Version
COMMIT_VAR=go.qbee.io/agent/app.Commit
COMMIT=$(shell git describe --always --dirty --abbrev=0)
GOOS=linux
GOARCH=amd64

# PUBLIC_SIGNING_KEY is a public part of platform's binary signing key.
# For production release, it must be replaced with the correct public key.
PUBLIC_SIGNING_KEY=xSHbUBG7LTuNfXd3zod4EX8_Es8FTCINgrjvx1WXFE4.plCHzlDAeb3IWW1wK6P6paMRYO4f8qceV3lrNCqNpWo
PUBLIC_SINGING_KEY_VAR=go.qbee.io/agent/app/binary.PublicSigningKey

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION) -X $(PUBLIC_SINGING_KEY_VAR)=$(PUBLIC_SIGNING_KEY) -X $(COMMIT_VAR)=$(COMMIT)" \
		-trimpath \
		-o bin/qbee-agent main.go

auto-build:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build; done

docker-image:
	docker build -t debian:qbee \
		--build-arg version=2023.01 \
		--build-arg public_signing_key=$(PUBLIC_SIGNING_KEY) \
		.

test:
	go test ./app/...

lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:v1.55.2 golangci-lint run