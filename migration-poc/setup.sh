#!/bin/bash

# Setup script for Network Migration POC
# This script checks and installs all required dependencies

set -e

echo "========================================"
echo "Network Migration POC - Setup Script"
echo "========================================"
echo ""

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check Python
echo "Checking Python installation..."
if command -v python3 &> /dev/null; then
    PYTHON_VERSION=$(python3 --version)
    echo -e "${GREEN}✓${NC} Python found: $PYTHON_VERSION"
else
    echo -e "${RED}✗${NC} Python3 not found!"
    echo "Please install Python 3.8 or higher"
    exit 1
fi

# Check pip
echo "Checking pip installation..."
if command -v pip3 &> /dev/null; then
    echo -e "${GREEN}✓${NC} pip3 found"
else
    echo -e "${RED}✗${NC} pip3 not found!"
    echo "Please install pip3"
    exit 1
fi

# Install/upgrade pip packages
echo ""
echo "Installing Python dependencies..."
echo ""

# Core packages
echo "→ Installing robotframework..."
pip3 install --quiet robotframework==7.0
echo -e "${GREEN}✓${NC} robotframework installed"

echo "→ Installing pyyaml..."
pip3 install --quiet pyyaml==6.0.1
echo -e "${GREEN}✓${NC} pyyaml installed"

# Optional but recommended
echo "→ Installing robotframework-metrics (optional)..."
pip3 install --quiet robotframework-metrics==3.3.3 2>/dev/null || echo -e "${YELLOW}⚠${NC} robotframework-metrics skipped (optional)"

# Verify installations
echo ""
echo "Verifying installations..."
echo ""

# Check Robot Framework
if python3 -c "import robot" 2>/dev/null; then
    ROBOT_VERSION=$(robot --version | head -1)
    echo -e "${GREEN}✓${NC} Robot Framework: $ROBOT_VERSION"
else
    echo -e "${RED}✗${NC} Robot Framework installation failed"
    exit 1
fi

# Check PyYAML
if python3 -c "import yaml" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} PyYAML: installed"
else
    echo -e "${RED}✗${NC} PyYAML installation failed"
    exit 1
fi

# Check Go (optional, only needed for building)
echo ""
echo "Checking Go installation (for building)..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version)
    echo -e "${GREEN}✓${NC} $GO_VERSION"
else
    echo -e "${YELLOW}⚠${NC} Go not found (only needed for building from source)"
fi

# Check network connectivity to lab devices
echo ""
echo "Checking network connectivity to lab devices..."
echo ""

# Test ping to first device
if ping -c 1 -W 2 172.10.1.1 &> /dev/null; then
    echo -e "${GREEN}✓${NC} Can reach UPE1 (172.10.1.1)"
else
    echo -e "${YELLOW}⚠${NC} Cannot reach UPE1 (172.10.1.1) - check network connectivity"
fi

if ping -c 1 -W 2 172.10.1.201 &> /dev/null; then
    echo -e "${GREEN}✓${NC} Can reach SR201 (172.10.1.201)"
else
    echo -e "${YELLOW}⚠${NC} Cannot reach SR201 (172.10.1.201) - check network connectivity"
fi

# Summary
echo ""
echo "========================================"
echo "Setup Summary"
echo "========================================"
echo ""
echo -e "${GREEN}✓${NC} Python dependencies installed"
echo -e "${GREEN}✓${NC} Robot Framework ready"
echo -e "${GREEN}✓${NC} PyYAML ready"
echo ""
echo "Next steps:"
echo "1. Make sure Go server is running: ./build/network-library-linux-amd64"
echo "2. Update credentials in: robot-tests/data/devices.yaml"
echo "3. Run tests: cd robot-tests && robot testcases/poc_test.robot"
echo ""
echo "========================================"
