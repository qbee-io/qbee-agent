build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/qbee-agent cmd/agent/main.go

auto-build:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build; done

docker-image:
	docker build -t debian:qbee .

auto-docker-image:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build docker-image; done
