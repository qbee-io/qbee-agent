#!/bin/bash
set -e

echo "Starting RAUC bundle creation..."
OUTPUT_DIR="/workspace/artifacts"
mkdir -p "$OUTPUT_DIR"

# 1. Generate test keys
echo "Generating test keys..."
openssl req -x509 -newkey rsa:4096 -nodes \
    -keyout "$OUTPUT_DIR/test.key" -out "$OUTPUT_DIR/test.cert" \
    -subj "/O=Test-Company/CN=Test-CA" -days 365

for version in 1.0.0 2.0.0; do

    BUNDLE_DIR=$(mktemp -d)

    # 2. Prepare payload
    mkdir -p "$BUNDLE_DIR"
    echo "Creating raw payload image..."
    dd if=/dev/urandom of="$BUNDLE_DIR/payload.img" bs=1M count=16 status=none
    mkfs.ext4 -q "$BUNDLE_DIR/payload.img"

    # 3. Create Manifest
    cat <<EOF > "$BUNDLE_DIR/manifest.raucm"
    [update]
compatible=Docker-Test-Device
version=$version

[bundle]
format=plain

[image.appfs]
filename=payload.img
EOF

    # 4. Generate Bundle
    echo "Building the RAUC bundle..."
    rauc bundle --cert="$OUTPUT_DIR/test.cert" \
        --key="$OUTPUT_DIR/test.key" \
        --keyring="$OUTPUT_DIR/test.cert" \
        "$BUNDLE_DIR" \
        "$OUTPUT_DIR/test-bundle-$version.raucb"
done

# Delete the private key for security reasons
rm -f "$OUTPUT_DIR/test.key"