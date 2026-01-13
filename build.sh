#!/bin/bash
# Build script for Connected Devices Server
# Builds for Oracle VM (Linux/arm64)
# Usage: ./build.sh [debug|prod|both]

set -e

PROJECT_NAME="server_app"
TARGET_OS="linux"
TARGET_ARCH="arm64"

# Default to building both
BUILD_TYPE="${1:-both}"

echo "=== Connected Devices Server Build Script ==="
echo "Target: $TARGET_OS/$TARGET_ARCH"
echo ""

# Clean previous builds
echo "Cleaning previous builds..."
rm -f "${PROJECT_NAME}_debug" "${PROJECT_NAME}_prod"
echo "✓ Clean complete"
echo ""

# Build Debug
if [[ "$BUILD_TYPE" == "debug" || "$BUILD_TYPE" == "both" ]]; then
    echo "Building DEBUG version..."
    GOOS=$TARGET_OS GOARCH=$TARGET_ARCH go build -tags debug -o "${PROJECT_NAME}_debug" -v
    chmod +x "${PROJECT_NAME}_debug"
    SIZE=$(ls -lh "${PROJECT_NAME}_debug" | awk '{print $5}')
    echo "✓ Debug build complete: ${PROJECT_NAME}_debug ($SIZE)"
    echo ""
fi

# Build Production
if [[ "$BUILD_TYPE" == "prod" || "$BUILD_TYPE" == "both" ]]; then
    echo "Building PRODUCTION version..."
    GOOS=$TARGET_OS GOARCH=$TARGET_ARCH go build -o "${PROJECT_NAME}_prod" -v
    chmod +x "${PROJECT_NAME}_prod"
    SIZE=$(ls -lh "${PROJECT_NAME}_prod" | awk '{print $5}')
    echo "✓ Production build complete: ${PROJECT_NAME}_prod ($SIZE)"
    echo ""
fi

echo "=== Build Summary ==="
echo "Location: $(pwd)"
ls -lh "${PROJECT_NAME}"_* 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
echo ""
echo "To deploy to Oracle VM:"
echo "  scp -i <keyfile> ${PROJECT_NAME}_debug ubuntu@<vm-ip>:~/server_app/"
echo "  scp -i <keyfile> ${PROJECT_NAME}_prod ubuntu@<vm-ip>:~/server_app/"
echo ""
echo "To run on VM:"
echo "  ssh ubuntu@<vm-ip> '~/server_app/${PROJECT_NAME}_debug'"
