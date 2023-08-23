#!/usr/bin/env bash

if [[ -z $BOOTSTRAP_KEY ]]; then
  echo "ERROR: No bootstrap key has been provided"
  exit 1
fi

BASEDIR=$(cd $(dirname $0) && pwd)

MAC=$(echo $HOSTNAME | md5sum | sed 's/^\(..\)\(..\)\(..\)\(..\)\(..\).*$/02:\1:\2:\3:\4:\5/')

generate_user_password() {
  < /dev/urandom tr -dc A-Z-a-z-0-9 | head -c 8
  echo
}

export QBEE_DEMO_USER="qbee"
export QBEE_DEMO_PASSWORD="qbee"
export QBEE_DEMO_BOOTSTRAP_KEY="${BOOTSTRAP_KEY}"
export QBEE_DEMO_PASSWORD_HASH=$(echo $QBEE_DEMO_PASSWORD | mkpasswd --method=SHA-512 --stdin)
export QBEE_DEMO_DEVICE_HUB_HOST=${QBEE_DEMO_DEVICE_HUB_HOST:-device.app.qbee.io}
export QBEE_DEMO_VPN_SERVER=${QBEE_DEMO_VPN_SERVER:-vpn.app.qbee.io}

envsubst > $BASEDIR/cloud-init/user-data < $BASEDIR/cloud-init/user-data.template

cloud-localds $BASEDIR/cloud-init/seed.img $BASEDIR/cloud-init/user-data $BASEDIR/cloud-init/meta-data

IMG="$BASEDIR/debian-12-generic-amd64.qcow2"
qemu-img resize $IMG 8G
  
QEMU_OPTIONS=""

if [[ -c /dev/kvm ]]; then
  QEMU_OPTIONS="$QEMU_OPTIONS -machine type=pc,accel=kvm -smp 4 -cpu host"
fi

qemu-system-x86_64 \
  -m 512 \
  -smp 4 \
  -nographic \
  -device virtio-net-pci,netdev=net0,mac=$MAC \
  -netdev user,id=net0,hostfwd=tcp::2222-:22 \
  -drive if=virtio,format=qcow2,file=$IMG \
  -drive if=virtio,format=raw,file=$BASEDIR/cloud-init/seed.img \
  $QEMU_OPTIONS

