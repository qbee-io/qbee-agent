VERSION=0000.00
VERSION_VAR=github.com/qbee-io/qbee-agent/app.Version

# PUBLIC_SIGNING_KEY is a public part of services/devicehub/cmd/agent-upload/dev-signing.key
# For production release, it must be replaced with the correct public key.
# Use cmd/get-public-signing-key/main.go to obtain a public key from a private key.
PUBLIC_SIGNING_KEY=xSHbUBG7LTuNfXd3zod4EX8_Es8FTCINgrjvx1WXFE4.plCHzlDAeb3IWW1wK6P6paMRYO4f8qceV3lrNCqNpWo
PUBLIC_SINGING_KEY_VAR=github.com/qbee-io/qbee-agent/app/updater.PublicSigningKey

build:
	docker run --rm \
	-v .:/src \
	-w /src \
	-e CGO_ENABLED=0 \
	golang:1.20 \
	go build \
		-ldflags "-s -w -X $(VERSION_VAR)=$(VERSION) -X $(PUBLIC_SINGING_KEY_VAR)=$(PUBLIC_SIGNING_KEY)" \
		-o bin/qbee-agent cmd/agent/main.go

auto-build:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build; done

docker-image:
	docker build -t debian:qbee .

auto-docker-image:
	inotifywait -e close_write,moved_to,create -r -q -m app/ | while read line; do $(MAKE) build docker-image; done

test:
	go test ./app/...