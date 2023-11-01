# qbee-agent v2

qbee-agent is the software running on Linux devices which are managed by the [qbee.io](https://qbee.io) IoT device management platform. For more information about the platform, please see our [documentation](https://qbee.io/docs/).

# License

The qbee-agent is licensed under the Apache License, Version 2.0. See
[LICENSE](https://github.com/qbee-io/qbee-agent/blob/master/LICENSE) for the full license text.

# Releasing new version

## Prerequisites

Go version 1.20 or higher (use the go version manager: https://github.com/moovweb/gvm)

```bash
$ go version
go version go1.20.3 linux/amd64

```

fpm (https://github.com/jordansissel/fpm)

```bash
sudo apt install ruby
sudo gem install fpm
```

Other dependencies (some usually installed on Linux systems already):

```bash
sudo apt install awscli gzip coreutils make
```

Install the gh cli
```bash
sudo snap install gh
```

## Obtain the signing key

Obtain the signing key from a secure location.

## Build packages

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

## Uploading packages

Make sure you have your AWS environment credentials for production

```bash
env | grep AWS_
[...]
AWS_SECRET_ACCESS_KEY=<AWS_SECRET_ACCESS_KEY>
AWS_ACCESS_KEY_ID=<AWS_ACCESS_KEY_ID>
[...]
```

Upload packages to our S3 backed CloudFront. If the VERSION is prefixed with ^20 we overwrite move the latest version pointer 
(https://cdn.qbee.io/software/qbee-agent/latest.txt) to point to this version. We are also uploading the release to GitHub. 
Remember to to log into github first.

```bash
gh auth login
```

```bash
./script/upload-release 
```

Upload binaries pointing to the signing key (make sure you have built the openvpn binaries first, look at README.md in apps/agent-v1/static)

```bash
./script/upload-updates /path/to/signing.key
```
