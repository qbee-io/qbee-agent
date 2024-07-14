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
	docker build -t rhel:qbee \
		--build-arg version=2023.01 \
		-f Dockerfile.rhel9 .

	docker build -t openwrt:qbee \
		--build-arg version=2023.01 \
		-f Dockerfile.openwrt .

test:
	go test ./app/...

lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:v1.55.2 golangci-lint run

build-win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION) -X $(COMMIT_VAR)=$(COMMIT)" \
		-trimpath \
		-o bin/qbee-agent-windows-amd64.exe main.go

build-mac:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION) -X $(COMMIT_VAR)=$(COMMIT)" \
		-trimpath \
		-o bin/qbee-agent-darwin-amd64 main.go

build-freebsd:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION) -X $(COMMIT_VAR)=$(COMMIT)" \
		-trimpath \
		-o bin/qbee-agent-freebsd-amd64 main.go