#!/bin/bash

# Diagnostic script to test Go server connection

echo "========================================"
echo "Go Remote Library - Diagnostics"
echo "========================================"
echo ""

# Check if server is running
echo "1. Checking if server is running..."
if lsof -i :8270 > /dev/null 2>&1; then
    PID=$(lsof -ti :8270)
    echo "   ✓ Server running (PID: ${PID})"
else
    echo "   ✗ Server NOT running"
    echo "   Start with: ./server.sh start"
    exit 1
fi

# Check port binding
echo ""
echo "2. Checking port binding..."
netstat -an | grep 8270

# Test TCP connection
echo ""
echo "3. Testing TCP connection to port 8270..."
if timeout 2 bash -c 'cat < /dev/null > /dev/tcp/localhost/8270' 2>/dev/null; then
    echo "   ✓ TCP connection successful"
else
    echo "   ✗ TCP connection failed"
    echo "   Server may be crashed or not accepting connections"
fi

# Try to connect with netcat
echo ""
echo "4. Testing with netcat (if available)..."
if command -v nc &> /dev/null; then
    echo '{"method":"get_keyword_names","args":[],"kwargs":{}}' | nc -w 2 localhost 8270 2>&1 | head -5
else
    echo "   netcat not available, skipping"
fi

# Check server process details
echo ""
echo "5. Server process details..."
ps aux | grep network-library | grep -v grep

# Check if server log exists
echo ""
echo "6. Checking server logs..."
if [ -f /tmp/go-library.log ]; then
    echo "   Server log (last 20 lines):"
    echo "   ----------------------------"
    tail -20 /tmp/go-library.log
else
    echo "   No log file found at /tmp/go-library.log"
fi

# Test with Python
echo ""
echo "7. Testing connection with Python..."
python3 << 'PYEOF'
import socket
import json
import sys

try:
    # Create socket
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(5)
    
    # Connect
    print("   → Connecting to localhost:8270...")
    sock.connect(('localhost', 8270))
    print("   ✓ Connected successfully")
    
    # Send request
    request = {"method": "get_keyword_names", "args": [], "kwargs": {}}
    print("   → Sending request...")
    sock.sendall((json.dumps(request) + '\n').encode())
    
    # Receive response
    print("   → Waiting for response...")
    response = sock.recv(4096)
    print("   ✓ Received response:")
    print("   ", response.decode()[:200])
    
    sock.close()
    print("   ✓ Connection test PASSED")
    
except socket.timeout:
    print("   ✗ Connection TIMEOUT - server not responding")
    sys.exit(1)
except ConnectionRefusedError:
    print("   ✗ Connection REFUSED - server not accepting connections")
    sys.exit(1)
except Exception as e:
    print(f"   ✗ Error: {e}")
    sys.exit(1)
PYEOF

PYTHON_RESULT=$?

echo ""
echo "========================================"
echo "Diagnostic Summary"
echo "========================================"
echo ""

if [ $PYTHON_RESULT -eq 0 ]; then
    echo "✓ Server is working correctly!"
    echo ""
    echo "If Robot Framework still fails, the issue is with:"
    echo "  - Robot Framework Remote library installation"
    echo "  - Or test file syntax"
    echo ""
    echo "Try reinstalling Robot Framework:"
    echo "  pip3 uninstall robotframework"
    echo "  pip3 install robotframework==7.0"
else
    echo "✗ Server connection test FAILED"
    echo ""
    echo "Recommended actions:"
    echo "  1. Stop server: ./server.sh stop"
    echo "  2. Rebuild binary: ./rebuild.sh"
    echo "  3. Start server: ./server.sh start"
    echo "  4. Run diagnostics again: ./diagnostics.sh"
fi

echo ""
