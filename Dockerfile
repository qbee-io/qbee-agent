FROM golang:1.20 as builder

ARG version
ENV VERSION_VAR=github.com/qbee-io/qbee-agent/app.Version

ARG public_signing_key
ENV PUBLIC_SINGING_KEY_VAR=github.com/qbee-io/qbee-agent/app/binary.PublicSigningKey

ENV CGO_ENABLED=0

WORKDIR /src

COPY . /src

# build the agent (with two different versions, so we can test auto-update mechanism)
RUN --mount=type=cache,target=/go \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -ldflags "-s -w -X ${VERSION_VAR}=$version -X ${PUBLIC_SINGING_KEY_VAR}=$public_signing_key" \
    -o /usr/sbin/qbee-agent.$version \
    cmd/agent/main.go && \
    go build \
    -ldflags "-s -w -X ${PUBLIC_SINGING_KEY_VAR}=$public_signing_key" \
    -o /usr/sbin/qbee-agent \
    cmd/agent/main.go

FROM debian:stable

ARG version

# add qbee-dev apt repo
COPY app/software/test_repository/debian /apt-repo
RUN echo "deb [trusted=yes] file:/apt-repo ./" > /etc/apt/sources.list.d/qbee-dev.list

# add docker repo, so we can install it when needed (disable auth)
RUN echo 'Acquire::https { Verify-Peer "false" }' > /etc/apt/apt.conf.d/99verify-peer.conf
RUN echo "deb [trusted=yes] http://download.docker.com/linux/debian bullseye stable" \
    > /etc/apt/sources.list.d/docker.list

# update apt cache
RUN apt-get update && apt-get upgrade -y

# create empty agent configuration directory
RUN mkdir /etc/qbee && echo '{}' > /etc/qbee/qbee-agent.json

WORKDIR /app

# copy the agent
COPY --from=builder /usr/sbin/qbee-agent /usr/sbin/qbee-agent
COPY --from=builder /usr/sbin/qbee-agent.$version /usr/sbin/qbee-agent.$version
