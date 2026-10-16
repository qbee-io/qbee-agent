FROM debian:trixie-slim

# Install RAUC, OpenSSL, squashfs-tools, and utilities
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    rauc \
    openssl \
    ca-certificates \
    squashfs-tools \
    dbus \
    e2fsprogs \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Set up the working directory
WORKDIR /workspace

# Copy our bundling script into the container
COPY build-bundle.sh /workspace/
RUN chmod +x /workspace/build-bundle.sh

ENTRYPOINT ["/workspace/build-bundle.sh"]
