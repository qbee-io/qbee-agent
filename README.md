# qbee-agent v2

## Obtain the signing key

Obtain the signing key from a secure location.

## Build procedure

Build the packages pointing to our signing key. Package versions will by default be 0000.00. Set the VERSION environment
variable to override. We're currently building deb and rpm packages in addition to a tarball to be used in YOCTO or any custom
package builds.

```bash
export VERSION=<version>
./script/build-packages /path/to/signing.key
```

These packages can be tested without a general release

## Simple test of the packages

Run simple tests on the built packages (TBD)

## Release procedure

Make sure you have your AWS environment credentials for production

```bash
env | grep AWS_
[...]
AWS_SECRET_ACCESS_KEY=<AWS_SECRET_ACCESS_KEY>
AWS_ACCESS_KEY_ID=<AWS_ACCESS_KEY_ID>
[...]
```

Upload packages to our S3 backed CloudFront. If the VERSION is prefixed with ^20 we overwrite move the latest version pointer 
(https://cdn.qbee.io/software/qbee-agent/latest.txt) to point to this version.

```bash
./script/upload-release 
```

Upload binaries pointing to the signing key (make sure you have built the openvpn binaries first, look at README.md in apps/agent-v1/static)

```bash
./script/upload-updates /path/to/signing.key
```
