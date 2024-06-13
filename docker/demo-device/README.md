# qbee-demo docker image

# Building the qbee-demo docker image

```bash
docker build -t qbeeio/qbee-demo .
docker push qbeeio/qbee-demo
```

# Running the resulting image 

```bash
docker run -it -e BOOTSTRAP_KEY=<bootstrap-key> qbeeio/qbee-demo
```

# Running with test infra structure

```bash
docker run -it -e BOOTSTRAP_KEY=<bootstrap-key> \
  -e QBEE_DEMO_DEVICE_HUB_HOST=device.app.qbee-dev.qbee.io \
  qbeeio/qbee-demo
```

# Running with kvm support

Qemu in docker does not have access the the kvm kernel virtualization which makes the resulting
quite slow. However, you can use kvm acceleration if available on your host.


```bash
kvm-ok
```

If this command outputs the following

```
INFO: /dev/kvm exists
KVM acceleration can be used
```

Then you can start the docker container with

```bash
docker run --device /dev/kvm -it -e BOOTSTRAP_KEY=<bootstrap-key> qbeeio/qbee-demo
```
