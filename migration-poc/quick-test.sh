#!/bin/bash

# Quick test script to validate POC setup
# This script will:
# 1. Build the Go library
# 2. Start the server in background
# 3. Run Robot Framework tests
# 4. Show results

set -e

echo "========================================"
echo "POC Quick Test Script"
echo "========================================"

# Step 1: Build
echo ""
echo "Step 1: Building Go library..."
./build.sh

# Step 2: Start server
echo ""
echo "Step 2: Starting Go library server..."
./build/network-library-linux-amd64 &
SERVER_PID=$!
echo "Server started (PID: $SERVER_PID)"

# Wait for server to be ready
echo "Waiting for server to initialize..."
sleep 3

# Step 3: Run tests
echo ""
echo "Step 3: Running Robot Framework tests..."
echo ""
cd robot-tests

# Check if robot is installed
if ! command -v robot &> /dev/null; then
    echo "⚠️  Robot Framework not found!"
    echo "Install it with: pip install robotframework"
    kill $SERVER_PID
    exit 1
fi

# Run tests
robot --outputdir reports testcases/poc_test.robot
TEST_RESULT=$?

# Step 4: Show results
echo ""
echo "========================================"
echo "Test Results"
echo "========================================"

if [ $TEST_RESULT -eq 0 ]; then
    echo "✓ All tests PASSED!"
else
    echo "✗ Some tests FAILED"
fi

echo ""
echo "View detailed results:"
echo "  open reports/report.html"
echo ""

# Cleanup
echo "Stopping Go library server..."
kill $SERVER_PID

echo ""
echo "========================================"
echo "POC Test Completed"
echo "========================================"
