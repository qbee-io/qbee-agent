VERSION=0000.00
VERSION_VAR=github.com/qbee-io/qbee-agent/app.Version

build:
	CGO_ENABLED=0 go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION)" \
		-o bin/qbee-agent cmd/agent/main.go

auto-build:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build; done

docker-image:
	docker build -t debian:qbee .

auto-docker-image:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build docker-image; done

test:
	go test ./app/...