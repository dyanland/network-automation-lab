# 🔧 ERROR FIX: "No keyword with name 'Connect To Device' found"

## 🎯 Problem

You're getting this error:
```
No keyword with name 'Connect To Device' found.
```

This means Robot Framework cannot find the keyword in the library.

---

## ✅ Solution: Replace GoNetworkLibrary.py

### Step 1: Backup Old Library
```bash
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc/robot-tests/libraries

# Backup old version
cp GoNetworkLibrary.py GoNetworkLibrary.py.old
```

### Step 2: Replace with Enhanced Version
```bash
# Copy the new enhanced version
cp /path/to/GoNetworkLibrary_ENHANCED.py GoNetworkLibrary.py
```

Or manually copy the content of `GoNetworkLibrary_ENHANCED.py` to `libraries/GoNetworkLibrary.py`

### Step 3: Verify Library Loads
```bash
# Test if library loads without errors
python3 -c "import sys; sys.path.insert(0, 'libraries'); from GoNetworkLibrary import GoNetworkLibrary; lib = GoNetworkLibrary(); print('✓ Library loaded successfully')"
```

### Step 4: Check Available Keywords
```bash
python3 << 'EOF'
import sys
sys.path.insert(0, 'libraries')
from GoNetworkLibrary import GoNetworkLibrary

lib = GoNetworkLibrary()

# List all public methods (keywords)
keywords = [method for method in dir(lib) if not method.startswith('_')]
print(f"✓ Found {len(keywords)} keywords:")
for kw in sorted(keywords):
    print(f"  - {kw}")
EOF
```

**Expected output:**
```
✓ Found 20+ keywords:
  - close_connection
  - connect_to_device
  - execute_command
  - extract_interface_errors
  - extract_ios_version
  - extract_ping_success_rate
  - extract_route_count
  - extract_uptime
  - get_bgp_summary
  - get_bgp_summary_ios_xe
  - get_connection_info
  - get_interface_status
  - get_ospf_neighbors
  - is_ios_xe_device
  - is_ios_xr_device
  - parse_cpu_utilization
  - parse_interface_status_ios_xr
  - parse_memory_utilization
  - ping_test
```

---

## 🔍 What Was Missing in Old Library

The old `GoNetworkLibrary.py` from the POC had basic methods, but the revised test file needs **additional keywords** that weren't there:

### Missing Keywords (Now Added):
- ✅ `get_bgp_summary_ios_xe()` - For ASR903 BGP commands
- ✅ `parse_interface_status_ios_xr()` - Parse IOS-XR interface status
- ✅ `extract_ping_success_rate()` - Parse ping results
- ✅ `extract_route_count()` - Parse route counts
- ✅ `parse_cpu_utilization()` - Parse CPU output
- ✅ `parse_memory_utilization()` - Parse memory output
- ✅ `extract_ios_version()` - Parse IOS version
- ✅ `extract_uptime()` - Parse device uptime
- ✅ `extract_interface_errors()` - Parse interface errors

---

## 📁 File Structure Should Be:

```
migration-poc/
├── go-library/
│   └── main.go
├── build/
│   └── network-library-linux-amd64
├── robot-tests/
│   ├── testcases/
│   │   └── test_pre_migration_baseline_REVISED.robot
│   ├── data/
│   │   ├── devices.yaml
│   │   └── meralco_vrfs.yaml
│   ├── libraries/
│   │   └── GoNetworkLibrary.py    <-- REPLACE THIS FILE
│   ├── baseline/
│   └── reports/
└── server.sh
```

---

## 🧪 Test After Replacement

### Test 1: Verify Server is Running
```bash
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc
./server.sh status

# Should show:
# Go server is running (PID: XXXXX)
```

### Test 2: Test Library Connection
```bash
cd robot-tests

python3 << 'EOF'
import sys
sys.path.insert(0, 'libraries')
from GoNetworkLibrary import GoNetworkLibrary

# Create library instance
lib = GoNetworkLibrary(host='127.0.0.1', port=8270)
print("✓ Library created")

# Try to connect to a device (use your actual IP)
try:
    handle = lib.connect_to_device('172.10.1.9', 'ASR9906', 'admin', 'admin')
    print(f"✓ Connected! Handle: {handle}")
    
    # Try a simple command
    output = lib.execute_command(handle, 'show version | include Version')
    print(f"✓ Command executed")
    print(f"Output: {output[:100]}...")
    
    # Close connection
    lib.close_connection(handle)
    print("✓ Connection closed")
    
except Exception as e:
    print(f"✗ Error: {e}")
EOF
```

### Test 3: Run Single Test Case
```bash
cd robot-tests

# Run just the first test to verify library loads
robot --pythonpath libraries \
      --outputdir baseline \
      --test "PRE-001*" \
      testcases/test_pre_migration_baseline_REVISED.robot
```

**Expected output:**
```
PRE-001: Load Device Configuration    | PASS |
```

---

## 🎯 If Still Getting Errors

### Error: "Cannot connect to Go server"
```bash
# Check if server is really running
ps aux | grep network-library

# Check if port 8270 is listening
netstat -tulpn | grep 8270

# Restart server
./server.sh restart
```

### Error: "PyYAML not found"
```bash
pip3 install pyyaml
```

### Error: "Library import failed"
```bash
# Check Python can find the library
cd robot-tests
python3 -c "import sys; sys.path.insert(0, 'libraries'); import GoNetworkLibrary; print('OK')"
```

### Error: "SSH connection failed"
```bash
# Test SSH manually first
ssh admin@172.10.1.9

# Check if credentials are correct in test file:
nano testcases/test_pre_migration_baseline_REVISED.robot
# Find:
${USERNAME}    admin    # <-- Update this
${PASSWORD}    admin    # <-- Update this
```

---

## 📋 Quick Fix Checklist

- [ ] Go server is running on port 8270
- [ ] Old GoNetworkLibrary.py backed up
- [ ] New GoNetworkLibrary_ENHANCED.py copied to libraries/GoNetworkLibrary.py
- [ ] PyYAML installed (`pip3 install pyyaml`)
- [ ] Test file credentials updated (${USERNAME} and ${PASSWORD})
- [ ] devices.yaml has correct IPs
- [ ] Can SSH manually to devices
- [ ] Library test script runs successfully

---

## 🚀 After Fix - Run Full Test

```bash
cd robot-tests

robot --pythonpath libraries \
      --outputdir baseline \
      --name "Pre-Migration Baseline" \
      testcases/test_pre_migration_baseline_REVISED.robot

# View results
firefox baseline/report.html
```

---

**The enhanced library has ALL keywords needed by the revised test file!** 🎉
