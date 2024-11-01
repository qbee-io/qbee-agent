FROM golang:1.22 as builder

ARG version
ENV VERSION_VAR=go.qbee.io/agent/app.Version

ENV CGO_ENABLED=0

WORKDIR /src

COPY . /src

# build the agent
RUN --mount=type=cache,target=/go \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -ldflags "-s -w -X ${VERSION_VAR}=$version" \
    -o /usr/sbin/qbee-agent \
    main.go

FROM debian:stable

ARG version

# add qbee-dev apt repo
COPY test/resources/debian /apt-repo
RUN echo "deb [trusted=yes] file:/apt-repo/repo ./" > /etc/apt/sources.list.d/qbee-dev.list

# Install ca-certificates in latest version
RUN apt-get update && apt-get install -y ca-certificates curl

# add docker repo, so we can install it when needed (disable auth)
RUN echo 'Acquire::https { Verify-Peer "false" }' > /etc/apt/apt.conf.d/99verify-peer.conf
RUN echo "deb [trusted=yes] http://download.docker.com/linux/debian bullseye stable" \
    > /etc/apt/sources.list.d/docker.list

# update apt cache
RUN apt-get update && apt-get upgrade -y
RUN apt-get install docker-ce-cli podman -y 

# install docker cli
RUN apt-get install docker-ce-cli -y

# create empty agent configuration directory
RUN mkdir /etc/qbee && echo '{}' > /etc/qbee/qbee-agent.json

WORKDIR /app

# copy the agent
COPY --from=builder /usr/sbin/qbee-agent /usr/sbin/qbee-agent
