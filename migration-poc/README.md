# Network Migration POC - Go Remote Library + Robot Framework

## 🎯 Overview

This POC demonstrates the integration of:
- **Go Remote Library** - Fast, dependency-free network device automation
- **Robot Framework** - Human-readable test cases for validation

### Key Benefits

✅ **Single Binary Deployment** - No Python dependencies on field laptops  
✅ **Fast Execution** - Native Go performance  
✅ **Readable Tests** - Change Management Board can review  
✅ **Cross-Platform** - Works on Windows, Linux, macOS  
✅ **Easy Updates** - Test data separated from code  

---

## 🚀 Quick Start (3 Steps)

### Step 1: Build the Go Library

```bash
# Linux/macOS
cd migration-poc
./build.sh

# Windows
build.bat
```

This creates binaries in `./build/` directory:
- `network-library-windows-amd64.exe` (Windows)
- `network-library-linux-amd64` (Linux)
- `network-library-darwin-amd64` (macOS)

### Step 2: Start the Go Library Server

```bash
# Copy binary to your lab machine (choose appropriate version)
# Windows:
network-library-windows-amd64.exe

# Linux/macOS:
chmod +x network-library-linux-amd64
./network-library-linux-amd64
```

You should see:
```
==============================================================================
  Network Migration Go Remote Library
  Listening on port 8270
  Ready for Robot Framework connections
==============================================================================
```

### Step 3: Run Robot Framework Tests

```bash
# Update credentials in robot-tests/data/devices.yaml
# Then run tests:

cd robot-tests
robot testcases/poc_test.robot
```

---

## 📋 Test Suite Overview

The POC includes 10 test cases:

| Test ID | Description | Tags |
|---------|-------------|------|
| TEST-001 | Verify Go library connectivity | smoke |
| TEST-002 | Connect to Core Router UPE1 | connection, core |
| TEST-003 | Execute show version | command |
| TEST-004 | Get OSPF neighbors | ospf, routing |
| TEST-005 | Check BGP summary | bgp, routing |
| TEST-006 | Connect to Aggregation SR201 | connection, aggregation |
| TEST-007 | Execute command on ASR903 | command |
| TEST-008 | Ping test between devices | connectivity |
| TEST-009 | Check interface status | interface |
| TEST-010 | Multi-device health check | health-check |

---

## 🔧 Configuration

### Update Device Credentials

Edit `robot-tests/data/devices.yaml`:

```yaml
credentials:
  username: "your_username"  # Change this
  password: "your_password"  # Change this
```

### Your Lab Devices (from host_info.csv)

**Core Routers (ASR9906):**
- UPE1: 172.10.1.1
- UPE9: 172.10.1.9
- UPE21: 172.10.1.21
- UPE24: 172.10.1.24

**Aggregation Routers (ASR903):**
- SR201: 172.10.1.201
- SR202: 172.10.1.202
- SR203: 172.10.1.203
- SR204: 172.10.1.204

**Switch:**
- Switch1: 172.10.1.50 (Cat9300)

---

## 📊 Test Reports

Robot Framework automatically generates:

1. **log.html** - Detailed test execution log
2. **report.html** - High-level summary with statistics
3. **output.xml** - Machine-readable results

View reports:
```bash
# Open in browser
open report.html  # macOS
xdg-open report.html  # Linux
start report.html  # Windows
```

---

## 🎨 Example Test Case

```robot
*** Test Cases ***
Verify Core Router OSPF
    [Documentation]    Check OSPF neighbors on core router
    
    # Connect using Go library (no Python SSH!)
    ${handle}=    GoLib.Connect To Device
    ...    172.10.1.1
    ...    ASR9906
    ...    admin
    ...    password
    
    # Get OSPF neighbors (parsed by Go)
    ${neighbors}=    GoLib.Get OSPF Neighbors    ${handle}
    
    # Validate in Robot Framework (readable!)
    Length Should Be    ${neighbors}    4
    
    FOR    ${neighbor}    IN    @{neighbors}
        Should Be Equal    ${neighbor}[state]    FULL
    END
```

**Output:**
```
[ PASS ] Verify Core Router OSPF (2.34s)
✓ Found 4 OSPF neighbors
✓ All neighbors in FULL state
```

---

## 🔍 Available Keywords

The Go library provides these keywords:

| Keyword | Arguments | Description |
|---------|-----------|-------------|
| `Connect To Device` | hostname, type, user, pass | Establish SSH connection |
| `Execute Command` | handle, command | Run CLI command |
| `Get OSPF Neighbors` | handle | Get parsed OSPF neighbor data |
| `Get BGP Summary` | handle, [vrf] | Get BGP peer status |
| `Get Interface Status` | handle, interface | Check interface state |
| `Ping Test` | handle, target, vrf, [count] | Connectivity test |
| `Close Connection` | handle | Close SSH connection |

---

## 🧪 Testing Strategy

### Pre-Migration Tests
```bash
robot --include baseline testcases/poc_test.robot
```

### During Migration
```bash
robot --include connectivity testcases/poc_test.robot
```

### Post-Migration
```bash
robot --include routing testcases/poc_test.robot
```

### Run Specific Tests
```bash
# Run only OSPF tests
robot --include ospf testcases/poc_test.robot

# Run only BGP tests
robot --include bgp testcases/poc_test.robot

# Run critical tests only
robot --include critical testcases/poc_test.robot
```

---

## 🐛 Troubleshooting

### Go Library Won't Start

**Problem:** Port 8270 already in use  
**Solution:** 
```bash
# Check what's using port 8270
netstat -an | grep 8270

# Kill existing process or use different port
```

### Can't Connect to Devices

**Problem:** SSH connection timeout  
**Check:**
1. Device IP reachable? `ping 172.10.1.1`
2. SSH enabled on device? `ssh admin@172.10.1.1`
3. Credentials correct in `devices.yaml`?
4. Firewall blocking port 22?

### Robot Framework Not Found

**Problem:** `robot: command not found`  
**Solution:**
```bash
pip install robotframework
pip install robotframework-sshlibrary  # Optional, not used in this POC
```

---

## 📦 Deployment to Field Engineers

### What They Need:

1. **Single binary file** (no dependencies!)
   - Copy `network-library-windows-amd64.exe` to their laptop
   
2. **Robot Framework** (only if running tests)
   ```bash
   pip install robotframework
   ```

3. **Test files** (optional)
   - Copy `robot-tests/` directory

### Running in Field:

```bash
# On their laptop:
1. Start library: network-library-windows-amd64.exe
2. Run tests: robot testcases/poc_test.robot
```

**That's it!** No Python SSH libraries, no dependency hell, just works.

---

## 🎯 Next Steps After POC

### If POC is Successful:

1. **Add More Test Cases**
   - VRF validation
   - MPLS forwarding checks
   - Segment Routing validation
   - Interface traffic analysis

2. **Create Migration-Specific Suites**
   - `pre_migration_baseline.robot`
   - `post_migration_validation.robot`
   - `rollback_verification.robot`

3. **Add Excel Reporting**
   - Generate detailed Excel reports
   - Compare pre/post states
   - Track migration metrics

4. **CI/CD Integration**
   - Automated testing on Git commit
   - Scheduled health checks
   - Change ticket integration

---

## 📝 Architecture

```
┌─────────────────────────────────────────┐
│   Robot Framework Test Cases           │
│   (Human-readable .robot files)        │
└──────────────┬──────────────────────────┘
               │ JSON-RPC over TCP
               │ (Port 8270)
┌──────────────▼──────────────────────────┐
│   Go Remote Library Server              │
│   - SSH connections (IOS-XR, IOS-XE)   │
│   - Command parsing                     │
│   - Data validation                     │
└──────────────┬──────────────────────────┘
               │ SSH (Port 22)
               │
┌──────────────▼──────────────────────────┐
│   Network Devices                       │
│   - ASR9906 (Core)                     │
│   - ASR903 (Aggregation)               │
│   - Cat9300 (Switch)                   │
└─────────────────────────────────────────┘
```

---

## 🆚 Comparison: Pure Python vs Go Library

| Aspect | Pure Python | Go Library |
|--------|-------------|------------|
| Dependencies | ❌ paramiko, netmiko, etc. | ✅ None (single binary) |
| Deployment | ❌ pip install on each machine | ✅ Copy one file |
| Speed | ⚠️ Moderate | ✅ Fast |
| Memory | ⚠️ ~50MB (Python + libs) | ✅ ~10MB |
| Startup | ⚠️ 2-3 seconds | ✅ Instant |
| Version conflicts | ❌ Common problem | ✅ Not possible |
| Cross-platform | ⚠️ Different behavior | ✅ Consistent |

---

## 📞 Support

For issues or questions:
1. Check the troubleshooting section above
2. Review Robot Framework logs: `log.html`
3. Check Go library console output
4. Enable debug mode: `robot --loglevel DEBUG`

---

## ✅ Success Criteria for POC

This POC is successful if:
- ✅ Go binary runs without Python dependencies
- ✅ Robot Framework connects to Go library
- ✅ Can connect to at least one device (UPE1 or SR201)
- ✅ OSPF/BGP data retrieved and parsed
- ✅ Tests pass with clear PASS/FAIL status
- ✅ Reports generated (log.html, report.html)

---

**Ready to test? Let's go! 🚀**

```bash
./build.sh
./build/network-library-linux-amd64 &
cd robot-tests
robot testcases/poc_test.robot
```
