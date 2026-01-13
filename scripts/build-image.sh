#!/bin/bash
# scripts/build-image.sh
# Build the Tanuki agent container image

set -e

IMAGE_NAME="${IMAGE_NAME:-bkonkle/tanuki}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Building ${IMAGE_NAME}:${IMAGE_TAG}..."

docker build \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -f "${PROJECT_DIR}/Dockerfile" \
    "${PROJECT_DIR}"

echo "Done! Run with:"
echo "  docker run -it ${IMAGE_NAME}:${IMAGE_TAG}"
