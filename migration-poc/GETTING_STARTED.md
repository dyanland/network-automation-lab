# Getting Started - Step by Step Guide

## Current Situation Analysis

Based on your screenshots, you have:
- ✅ Setup completed successfully (Python, Robot Framework, PyYAML installed)
- ✅ Port 8270 is listening (server is running somewhere)
- ❌ Problem: Multiple server instances or wrong directory

---

## 🔧 Step-by-Step Fix

### Step 1: Clean Up Any Running Servers

```bash
# Kill all running instances
pkill network-library

# Verify they're stopped
ps aux | grep network-library

# Check port is free
netstat -an | grep 8270
# Should show nothing now
```

### Step 2: Navigate to Project Root

```bash
# Go to project root
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc

# Verify you're in the right place
pwd
# Should show: /home/cisco/Pre_Post/network-automation-lab/migration-poc

# Check build directory exists
ls -la build/
# Should show: network-library-linux-amd64
```

### Step 3: Start Server Correctly

```bash
# Option A: Using the full path (RECOMMENDED)
./build/network-library-linux-amd64 &

# Option B: Using server management script (BEST)
./server.sh start

# Wait for startup
sleep 3
```

### Step 4: Verify Server is Running

```bash
# Check process
ps aux | grep network-library | grep -v grep

# Check port
netstat -an | grep 8270
# Should show: tcp    0    0 :::8270    :::*    LISTEN

# Check status (if using server.sh)
./server.sh status
```

### Step 5: Update Credentials

```bash
# Edit devices.yaml with your credentials
vim robot-tests/data/devices.yaml

# Change these lines:
credentials:
  username: "admin"      # ← Your actual username
  password: "admin"      # ← Your actual password
```

### Step 6: Run Tests

```bash
# Navigate to robot-tests directory
cd robot-tests

# Run the POC test suite
robot testcases/poc_test.robot
```

---

## 🎯 Complete Command Sequence (Copy-Paste)

Here's everything in one block you can copy-paste:

```bash
# 1. Clean up
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc
pkill network-library
sleep 2

# 2. Start server
./build/network-library-linux-amd64 > /tmp/go-server.log 2>&1 &
sleep 3

# 3. Verify server
netstat -an | grep 8270
echo "Server should be listening above ↑"

# 4. Run tests
cd robot-tests
robot testcases/poc_test.robot
```

---

## 🐛 If Still Having Connection Issues

### Check 1: Is Server Really Running?

```bash
# Check process
ps aux | grep network-library

# You should see something like:
# root  64057  0.0  network-library-linux-amd64

# Check port binding
sudo netstat -tlnp | grep 8270

# You should see:
# tcp  0  0  :::8270  :::*  LISTEN  64057/network-lib
```

### Check 2: Can You Connect to Port 8270?

```bash
# Test connection
lsof -i :8270

# Should show:
# COMMAND     PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
# network-l 64057 root    3u  IPv6 406603      0t0  TCP *:8270 (LISTEN)
```

### Check 3: Check Server Logs

```bash
# If started with output to log file:
cat /tmp/go-server.log

# Should show:
# ==============================================================================
#   Network Migration Go Remote Library
#   Listening on port 8270
#   Ready for Robot Framework connections
# ==============================================================================
```

### Check 4: Test Minimal Connection

Create a minimal test file:

```bash
# Create minimal test
cat > /tmp/test_connection.robot << 'EOF'
*** Settings ***
Library    Remote    http://localhost:8270    WITH NAME    GoLib

*** Test Cases ***
Test Server Connection
    ${keywords}=    GoLib.Get Keyword Names
    Log    Server is working! Keywords: ${keywords}    console=yes
    Should Not Be Empty    ${keywords}
EOF

# Run minimal test
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc
robot /tmp/test_connection.robot
```

If this minimal test **passes** → Your server is fine, issue is in test files or credentials
If this minimal test **fails** → Server connection issue

---

## 🔍 Common Mistakes

### Mistake 1: Running from Wrong Directory

❌ **WRONG:**
```bash
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc/build
./network-library-linux-amd64  # Wrong! Can't find data files
```

✅ **CORRECT:**
```bash
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc
./build/network-library-linux-amd64  # Correct! Full path from project root
```

### Mistake 2: Multiple Server Instances

```bash
# Check for multiple instances
ps aux | grep network-library

# If you see multiple processes, kill them all:
pkill network-library

# Then start fresh:
./build/network-library-linux-amd64 &
```

### Mistake 3: Server Started but Crashed

```bash
# Check if server is still running after a few seconds
./build/network-library-linux-amd64 &
sleep 5
ps aux | grep network-library

# If not running, check logs
cat /tmp/go-server.log
```

---

## 📊 Expected Output

### When Starting Server:

```
root@jumphost:/home/cisco/Pre_Post/network-automation-lab/migration-poc# ./build/network-library-linux-amd64 &
[1] 64057

root@jumphost:/home/cisco/Pre_Post/network-automation-lab/migration-poc# 
==============================================================================
  Network Migration Go Remote Library
  Listening on port 8270
  Ready for Robot Framework connections
==============================================================================
```

### When Checking Port:

```
root@jumphost:/home/cisco/Pre_Post/network-automation-lab/migration-poc# netstat -an | grep 8270
tcp6       0      0 :::8270                 :::*                    LISTEN
```

### When Running Tests:

```
root@jumphost:/home/cisco/Pre_Post/network-automation-lab/migration-poc/robot-tests# robot testcases/poc_test.robot
==============================================================================
Poc Test :: POC Test Suite - Network Migration Validation
==============================================================================
===== POC TEST SUITE STARTING =====
Go Remote Library: localhost:8270
Username: admin
==========================================
TEST-001: Verify Go Remote Library Connection              | PASS |
Testing connection to Go Remote Library on port 8270
✓ Go Remote Library is responsive
------------------------------------------------------------------------------
TEST-002: Connect to Core Router UPE1                      | PASS |
Connecting to UPE1 (172.10.1.1)...
✓ Connected successfully, handle: conn_1
------------------------------------------------------------------------------
...
```

---

## 🎯 Quick Diagnosis Commands

Run these to diagnose your current state:

```bash
# Where am I?
pwd

# Is server binary here?
ls -la build/network-library-linux-amd64

# Is server running?
ps aux | grep network-library | grep -v grep

# Is port listening?
netstat -an | grep 8270

# Can I reach devices?
ping -c 2 172.10.1.1

# Are Python packages installed?
python3 -c "import robot, yaml; print('All deps OK')"
```

---

## 🆘 Emergency Reset

If everything is confused, start fresh:

```bash
# 1. Kill everything
pkill network-library

# 2. Go to project root
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc

# 3. Clean rebuild (if needed)
# ./build.sh

# 4. Start server fresh
./build/network-library-linux-amd64 > /tmp/go-server.log 2>&1 &

# 5. Wait and verify
sleep 3
./server.sh status

# 6. Run minimal test
cat > /tmp/test.robot << 'EOF'
*** Settings ***
Library    Remote    http://localhost:8270    WITH NAME    GoLib

*** Test Cases ***
Ping Server
    ${keywords}=    GoLib.Get Keyword Names
    Log    ${keywords}    console=yes
EOF

robot /tmp/test.robot

# 7. If minimal test passes, run full suite
cd robot-tests
robot testcases/poc_test.robot
```

---

## 📝 Summary of Your Issue

Based on your screenshots:

1. ✅ Setup script ran successfully
2. ✅ All Python dependencies installed
3. ✅ Port 8270 is listening
4. ❌ **But you're trying to run the binary from wrong location**

**The fix:** Always run from project root with full path:
```bash
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc
./build/network-library-linux-amd64 &
```

**Not:** 
```bash
cd somewhere/else
./network-library-linux-amd64  # This won't work!
```

---

Try the "Complete Command Sequence" section above and let me know if you still have issues!
