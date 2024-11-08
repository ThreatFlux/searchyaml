# Use a multi-stage build for efficiency
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    bash \
    tar \
    curl \
    gcc \
    musl-dev

# Set the working directory
WORKDIR /build

# Copy build script and make it executable
COPY build.sh .
RUN chmod +x build.sh

# Create directory for output
RUN mkdir -p /output

# Build argument for version
ARG VERSION=dev
ENV VERSION=${VERSION}

# Copy the source code
COPY . .

# Run the build script
RUN ./build.sh /output

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Install necessary runtime tools
RUN apk add --no-cache \
    bash \
    tar \
    ca-certificates

# Create a non-root user
RUN adduser -D -h /app appuser

# Create necessary directories
RUN mkdir -p /app/build && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser
WORKDIR /app

# Copy artifacts from builder
COPY --from=builder --chown=appuser:appuser /output /app/build

# Add the entrypoint script
COPY --chown=appuser:appuser <<EOF /app/entrypoint.sh
#!/bin/bash
set -e

# Function to create GitHub Actions output commands
set_output() {
    echo "::set-output name=$1::$2"
}

# Get the version
VERSION=\$(cat /app/build/version.txt)
set_output "version" "\$VERSION"

# Create a list of artifacts
ARTIFACTS=\$(ls -1 /app/build/* | tr '\n' ' ')
set_output "artifacts" "\$ARTIFACTS"

# If we're running in GitHub Actions, move artifacts to the specified location
if [ -n "\${GITHUB_WORKSPACE}" ]; then
    mkdir -p \${GITHUB_WORKSPACE}/artifacts
    cp -r /app/build/* \${GITHUB_WORKSPACE}/artifacts/
fi

# Print build info
echo "Build completed successfully!"
echo "Version: \$VERSION"
echo "Artifacts: \$ARTIFACTS"
EOF

RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]