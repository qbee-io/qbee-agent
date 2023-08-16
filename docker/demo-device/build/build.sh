#!/usr/bin/env bash
set -e

BASEDIR=$(cd $(dirname $0) && pwd)

IMG="$BASEDIR/debian-12-generic-amd64.qcow2"
IMG="$BASEDIR/debian-12-generic-amd64.qcow2"
MAC=$(echo $HOSTNAME | md5sum | sed 's/^\(..\)\(..\)\(..\)\(..\)\(..\).*$/02:\1:\2:\3:\4:\5/')

cloud-localds $BASEDIR/cloud-init/seed.img $BASEDIR/cloud-init/user-data

QEMU_OPTIONS="-accel tcg"

if [[ -c /dev/kvm ]]; then
  QEMU_OPTIONS="$QEMU_OPTIONS -machine type=pc,accel=kvm -smp 4 -cpu host"
fi

qemu-system-x86_64 \
  -m 1G \
  -nographic \
  -device virtio-net-pci,netdev=net0,mac=$MAC \
  -netdev user,id=net0,hostfwd=tcp::2222-:22 \
  -drive if=virtio,format=qcow2,file=$IMG \
  -drive if=virtio,format=raw,file=$BASEDIR/cloud-init/seed.img \
  $QEMU_OPTIONS
