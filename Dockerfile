FROM debian:stable

# add qbee-dev apt repo
COPY app/software/test_repository/debian /apt-repo
RUN echo "deb [trusted=yes] file:/apt-repo ./" > /etc/apt/sources.list.d/qbee-dev.list

# update apt cache
RUN apt-get update && apt-get upgrade -y

# create empty agent configuration directory
RUN mkdir /etc/qbee && echo '{}' > /etc/qbee/qbee-agent.json

# copy the agent
COPY bin/qbee-agent /usr/sbin/qbee-agent
