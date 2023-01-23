FROM debian:stable

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

# copy the agent
COPY bin/qbee-agent /usr/sbin/qbee-agent
