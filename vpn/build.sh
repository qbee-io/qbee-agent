#!/usr/bin/env bash

OPENVPN_VERSION=2.6.1

# update apt cache
apt-get update

# install build tools and required libraries
apt-get install -fy build-essential wget libssl-dev pkg-config libnl-genl-3-dev libcap-ng-dev

# download and build openvpn
wget -O /tmp/openvpn.tar.gz https://swupdate.openvpn.org/community/releases/openvpn-${OPENVPN_VERSION}.tar.gz
tar -xvzf /tmp/openvpn.tar.gz -C /tmp

(cd /tmp/openvpn-${OPENVPN_VERSION} && ./configure \
  --disable-lz4 \
  --disable-lzo \
  --enable-static \
  --disable-shared \
  --disable-debug \
  --disable-plugins \
  --enable-small && make)

# copy built binary into the current directory
cp /tmp/openvpn-${OPENVPN_VERSION}/src/openvpn/openvpn ./vpn