#!/usr/bin/env bash

echo "Setting up RAUC Daemon environment..."
mkdir -p /mnt/rauc
mkdir -p /etc/rauc

# Create System Configuration
cat <<EOF > /etc/rauc/system.conf
[system]
compatible=Docker-Test-Device
bootloader=custom
mountprefix=/mnt/rauc
data-directory=/rauc

[keyring]
path=/rauc/test.cert

[handlers]
bootloader-custom-backend=/usr/local/bin/dummy-bootloader.sh

[slot.appfs.0]
device=/tmp/slot-a.img
type=raw
bootname=A

[slot.appfs.1]
device=/tmp/slot-b.img
type=raw
bootname=B
EOF

# Create dummy destination files
dd if=/dev/zero of=/tmp/slot-a.img bs=1M count=16 status=none
dd if=/dev/zero of=/tmp/slot-b.img bs=1M count=16 status=none

# Configure D-Bus and Mock the Kernel
echo "Setting up D-Bus..."
mkdir -p /var/run/dbus
mkdir -p /etc/dbus-1/system.d

cat <<EOF > /etc/dbus-1/system.d/rauc.conf
<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-BUS Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <policy user="root">
    <allow own="de.pengutronix.rauc"/>
    <allow send_destination="de.pengutronix.rauc"/>
  </policy>
  <policy context="default">
    <allow send_destination="de.pengutronix.rauc"/>
  </policy>
</busconfig>
EOF

# Fake the kernel command line
echo "rauc.slot=A" > /tmp/cmdline
mount --bind /tmp/cmdline /proc/cmdline

dbus-uuidgen > /var/lib/dbus/machine-id
dbus-daemon --system --fork

# Wait for D-Bus to be ready
echo "Waiting for D-Bus to be ready..."
for _ in {1..30}; do
  if dbus-send --system --print-reply --dest=org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.ListNames &>/dev/null; then
    echo "D-Bus is ready"
    break
  fi
  sleep 0.1
done

# Start the RAUC Daemon
rauc service > "/tmp/rauc-daemon.log" 2>&1 &

# Wait for RAUC service to be ready
echo "Waiting for RAUC service to be ready..."
for _ in {1..30}; do
  if rauc status &>/dev/null; then
    echo "RAUC service is ready"
    break
  fi
  sleep 0.1
done
