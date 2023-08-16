FROM alpine:latest as build

RUN apk add qemu-system-x86_64 cloud-utils-localds
ADD https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2 /build/debian-12-generic-amd64.qcow2
COPY build /build

RUN sh -x /build/build.sh

FROM alpine:latest
RUN apk add qemu-system-x86_64 cloud-utils-localds qemu-img envsubst
COPY --from=build /build/debian-12-generic-amd64.qcow2 /qbee-demo/debian-12-generic-amd64.qcow2
COPY /run /qbee-demo

CMD ["/bin/sh", "/qbee-demo/start.sh" ]
