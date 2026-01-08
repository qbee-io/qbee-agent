FROM golang:1.25.3 as builder

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

FROM debian:bookworm

ARG version

# add qbee-dev apt repo
COPY test/resources/debian /apt-repo
RUN echo "deb [trusted=yes] file:/apt-repo/repo ./" > /etc/apt/sources.list.d/qbee-dev.list

# Install ca-certificates in latest version
RUN apt-get update && \
  apt-get install -y ca-certificates curl && \
  install -m 0755 -d /etc/apt/keyrings && \
  curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc && \
  chmod a+r /etc/apt/keyrings/docker.asc

# add docker repo, so we can install it when needed (disable auth)
RUN echo 'Acquire::https { Verify-Peer "false" }' > /etc/apt/apt.conf.d/99verify-peer.conf
RUN echo "deb [trusted=yes] http://download.docker.com/linux/debian bookworm stable" \
    > /etc/apt/sources.list.d/docker.list

# Add Docker's official GPG key:

# Add the repository to Apt sources:
RUN echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null
# update apt cache
RUN apt-get update && apt-get upgrade -y
RUN apt-get install docker-ce-cli podman sudo -y

# create empty agent configuration directory
RUN mkdir /etc/qbee && echo '{}' > /etc/qbee/qbee-agent.json

WORKDIR /app

# copy the agent
COPY --from=builder /usr/sbin/qbee-agent /usr/sbin/qbee-agent

# add docker-compose files
COPY test/resources/docker-compose /docker-compose

# add an unprivileged user (with subuid/subgid ranges for rootless containers)
RUN adduser --system --group --home /var/lib/qbee-home qbee
RUN usermod --add-subuids 100000-165535 --add-subgids 100000-165535 qbee

# add sudoers file for qbee user
COPY test/resources/common/sudoers.d/99-qbee /etc/sudoers.d/99-qbee
