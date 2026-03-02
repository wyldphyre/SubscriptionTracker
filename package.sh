#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="subscription-tracker"
VERSION=$(date +%Y%m%d)
OUTPUT="${IMAGE_NAME}-${VERSION}.tar.gz"

echo "==> Building Linux binary (amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o subtracker .

echo "==> Building Docker image ${IMAGE_NAME}:${VERSION}..."
docker build -t "${IMAGE_NAME}:${VERSION}" -t "${IMAGE_NAME}:latest" .

echo "==> Removing Linux binary..."
rm subtracker

echo "==> Saving image to ${OUTPUT}..."
docker save "${IMAGE_NAME}:latest" | gzip > "${OUTPUT}"

echo ""
echo "Done! Transfer ${OUTPUT} to your server and run deploy.ps1."
echo "Image size: $(du -sh "${OUTPUT}" | cut -f1)"
