#!/bin/bash
# scripts/build-multiplatform.sh
# Build the Tanuki agent container image for multiple platforms

set -e

IMAGE_NAME="${IMAGE_NAME:-bkonkle/tanuki}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
PUSH="${PUSH:-false}"

echo "Building ${IMAGE_NAME}:${IMAGE_TAG} for linux/amd64 and linux/arm64..."

# Create builder if it doesn't exist
if ! docker buildx inspect tanuki-builder > /dev/null 2>&1; then
    echo "Creating buildx builder..."
    docker buildx create --name tanuki-builder --use
fi

# Use the builder
docker buildx use tanuki-builder

# Build command
BUILD_CMD="docker buildx build \
    --platform linux/amd64,linux/arm64 \
    -t ${IMAGE_NAME}:${IMAGE_TAG} \
    -f ${PROJECT_DIR}/Dockerfile"

if [ "$PUSH" = "true" ]; then
    BUILD_CMD="$BUILD_CMD --push"
    echo "Will push to registry after build..."
else
    BUILD_CMD="$BUILD_CMD --load"
    echo "Building locally (use PUSH=true to push to registry)..."
    echo "Note: --load only works for single platform. Building for current platform only."
    BUILD_CMD="docker buildx build \
        -t ${IMAGE_NAME}:${IMAGE_TAG} \
        -f ${PROJECT_DIR}/Dockerfile \
        --load"
fi

$BUILD_CMD "${PROJECT_DIR}"

echo "Done!"
