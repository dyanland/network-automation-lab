#!/bin/bash

# Rebuild script for Go Remote Library

echo "========================================"
echo "Rebuilding Go Remote Library"
echo "========================================"

PROJECT_ROOT="/home/cisco/Pre_Post/network-automation-lab/migration-poc"
cd "${PROJECT_ROOT}"

# Stop any running servers
echo "→ Stopping existing servers..."
pkill network-library
sleep 2

# Check Go installation
echo "→ Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo "✗ Go not found!"
    echo "Please install Go 1.21 or higher"
    exit 1
fi

GO_VERSION=$(go version)
echo "✓ Found: $GO_VERSION"

# Clean old build
echo "→ Cleaning old builds..."
rm -rf build/
mkdir -p build/

# Navigate to go-library directory
cd go-library

# Download dependencies
echo "→ Downloading Go dependencies..."
go mod download
go mod tidy

# Build for Linux
echo "→ Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../build/network-library-linux-amd64 main.go

# Check if build succeeded
if [ -f "../build/network-library-linux-amd64" ]; then
    echo "✓ Build successful!"
    ls -lh ../build/network-library-linux-amd64
    chmod +x ../build/network-library-linux-amd64
else
    echo "✗ Build failed!"
    exit 1
fi

cd ..

echo ""
echo "========================================"
echo "Build Complete!"
echo "========================================"
echo ""
echo "Next steps:"
echo "1. Start server: ./server.sh start"
echo "2. Run tests: cd robot-tests && robot testcases/poc_test.robot"
echo ""

