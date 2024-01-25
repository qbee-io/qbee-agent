VERSION=0000.00
VERSION_VAR=go.qbee.io/agent/app.Version
COMMIT_VAR=go.qbee.io/agent/app.Commit
COMMIT=$(shell git describe --always --dirty --abbrev=0)
GOOS=linux
GOARCH=amd64

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION) -X $(COMMIT_VAR)=$(COMMIT)" \
		-trimpath \
		-o bin/qbee-agent main.go

auto-build:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build; done

docker-image:
	docker build -t debian:qbee \
		--build-arg version=2023.01 \
		.

test:
	go test ./app/...
