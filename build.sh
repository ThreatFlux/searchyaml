#!/bin/bash

# Build script for SearchYAML

# Default build directory
BUILD_DIR=${1:-"build_searchyaml"}

# Version from git tag, fallback to dev version if no tag exists
VERSION=$(git describe --tags 2>/dev/null || echo "dev-$(git rev-parse --short HEAD)")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    log_error "Go is not installed. Please install Go before running this script."
    exit 1
fi

# Create build directory
mkdir -p "$BUILD_DIR"
log_info "Building SearchYAML version: $VERSION"
log_info "Output directory: $BUILD_DIR"

# Build function
build() {
    local OS=$1
    local ARCH=$2
    local OUTPUT="$BUILD_DIR/searchyaml-$VERSION-$OS-$ARCH"

    if [ "$OS" = "windows" ]; then
        OUTPUT="$OUTPUT.exe"
    fi

    log_info "Building for $OS/$ARCH..."

    # Set environment variables for cross-compilation
    GOOS=$OS GOARCH=$ARCH go build \
        -ldflags "-X main.Version=$VERSION -s -w" \
        -o "$OUTPUT" \
        2>/tmp/go-build-error.log

    if [ $? -eq 0 ]; then
        log_success "Built $OUTPUT successfully"
        # Create checksum
        if command -v sha256sum &> /dev/null; then
            sha256sum "$OUTPUT" > "$OUTPUT.sha256"
        elif command -v shasum &> /dev/null; then
            shasum -a 256 "$OUTPUT" > "$OUTPUT.sha256"
        fi
    else
        log_error "Failed to build for $OS/$ARCH"
        cat /tmp/go-build-error.log
        return 1
    fi
}

# Clean up any previous builds
if [ -d "$BUILD_DIR" ]; then
    log_info "Cleaning previous builds..."
    rm -rf "$BUILD_DIR"/*
fi

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    OS=${PLATFORM%/*}
    ARCH=${PLATFORM#*/}
    build "$OS" "$ARCH"
done

# If all builds succeeded, create a version file
echo "$VERSION" > "$BUILD_DIR/version.txt"
log_info "Build complete. Binaries are available in $BUILD_DIR/"

# List all built files
echo -e "\nBuilt files:"
ls -lh "$BUILD_DIR"

# Optional: Create tar archives for each binary
if command -v tar &> /dev/null; then
    log_info "Creating archives..."
    cd "$BUILD_DIR" || exit
    for file in searchyaml-*; do
        if [ ! -f "$file" ] || [[ "$file" == *.sha256 ]]; then
            continue
        fi
        tar czf "$file.tar.gz" "$file" "$file.sha256"
        log_success "Created $file.tar.gz"
    done
    cd - > /dev/null || exit
fi

log_success "Build process completed successfully!"