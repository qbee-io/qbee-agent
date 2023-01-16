FROM debian:stable

# add qbee-dev apt repo
RUN echo "deb [trusted=yes] http://qbee-dev-public.s3.eu-west-1.amazonaws.com/apt/amd64 /" \
    > /etc/apt/sources.list.d/qbee-dev.list

# create empty agent configuration directory
RUN mkdir /etc/qbee && echo '{}' > /etc/qbee/qbee-agent.json

# update apt cache
RUN apt-get update && apt-get upgrade -y

# copy the agent
COPY bin/qbee-agent /usr/sbin/qbee-agent
