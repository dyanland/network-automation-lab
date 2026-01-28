#!/bin/bash

# Build script for Go Remote Library
# Creates binaries for Windows, Linux, and macOS

set -e

PROJECT_NAME="network-library"
VERSION="1.0.0-poc"
BUILD_DIR="./build"

echo "========================================"
echo "Building Network Migration Go Library"
echo "Version: ${VERSION}"
echo "========================================"

# Clean build directory
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

cd go-library

# Download dependencies
echo ""
echo "→ Downloading dependencies..."
go mod download
go mod tidy

echo ""
echo "→ Building binaries..."

# Build for Windows (64-bit)
echo "  • Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../${BUILD_DIR}/${PROJECT_NAME}-windows-amd64.exe main.go

# Build for Linux (64-bit)
echo "  • Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../${BUILD_DIR}/${PROJECT_NAME}-linux-amd64 main.go

# Build for Linux (ARM64) - for newer devices
echo "  • Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../${BUILD_DIR}/${PROJECT_NAME}-linux-arm64 main.go

# Build for macOS (Intel)
echo "  • macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../${BUILD_DIR}/${PROJECT_NAME}-darwin-amd64 main.go

# Build for macOS (Apple Silicon)
echo "  • macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../${BUILD_DIR}/${PROJECT_NAME}-darwin-arm64 main.go

cd ..

echo ""
echo "✓ Build completed successfully!"
echo ""
echo "Built binaries:"
ls -lh ${BUILD_DIR}/

echo ""
echo "========================================"
echo "Deployment Instructions:"
echo "========================================"
echo "1. Copy the appropriate binary to your lab machine:"
echo "   - Windows: ${PROJECT_NAME}-windows-amd64.exe"
echo "   - Linux:   ${PROJECT_NAME}-linux-amd64"
echo "   - macOS:   ${PROJECT_NAME}-darwin-amd64 (or arm64)"
echo ""
echo "2. Make executable (Linux/macOS):"
echo "   chmod +x ${PROJECT_NAME}-*"
echo ""
echo "3. Run the server:"
echo "   ./${PROJECT_NAME}-linux-amd64"
echo ""
echo "4. Run Robot Framework tests:"
echo "   robot robot-tests/testcases/poc_test.robot"
echo "========================================"
