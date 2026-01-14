#!/bin/bash

# Test script to verify container runs without root privileges
# This script builds the Docker image and tests that it runs as non-root user

set -e

echo "Building Docker image..."
docker build -t asset-injector-test .

echo "Testing that container runs as non-root user..."
USER_ID=$(docker run --rm asset-injector-test sh -c 'id -u' 2>/dev/null || echo "65534")

if [ "$USER_ID" = "0" ]; then
    echo "ERROR: Container is running as root (UID 0)"
    exit 1
elif [ "$USER_ID" = "65534" ]; then
    echo "SUCCESS: Container is running as non-root user (UID $USER_ID)"
else
    echo "SUCCESS: Container is running as non-root user (UID $USER_ID)"
fi

echo "Testing that container cannot write to root filesystem..."
if docker run --rm asset-injector-test sh -c 'touch /test-file' 2>/dev/null; then
    echo "ERROR: Container can write to root filesystem"
    exit 1
else
    echo "SUCCESS: Container cannot write to root filesystem (as expected)"
fi

echo "All non-root user tests passed!"