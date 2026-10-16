## Overview
This directory contains scripts and docker environments to build rauc bundle artifacts
set up a docker container for running rauc tests.

## Build the rauc bundler
```
docker build -t rauc-bundler .
```

## Build a rauc bundle for testing
```
docker run --rm -v $(pwd)/artifacts:/workspace/artifacts rauc-bundler
```
