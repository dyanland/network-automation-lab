# Quick Reference Card - Network Migration POC

## 🚀 30-Second Quick Start

```bash
# 1. Build (one time)
./build.sh

# 2. Start server (keep running)
./build/network-library-linux-amd64

# 3. Run tests (in another terminal)
cd robot-tests
robot testcases/poc_test.robot

# 4. View results
open report.html
```

---

## 📋 Project Files at a Glance

```
migration-poc/
├── README.md                 ← Start here!
├── LAB_TESTING_GUIDE.md     ← Step-by-step lab guide
├── PROJECT_STRUCTURE.md      ← Architecture details
├── build.sh                  ← Build script
├── go-library/main.go        ← The Go magic (500 lines)
└── robot-tests/
    ├── data/devices.yaml     ← Update credentials here!
    └── testcases/
        ├── poc_test.robot    ← 10 basic tests
        └── advanced_migration.robot ← Real migration tests
```

---

## ⚡ Most Important Commands

### Build for Your Platform
```bash
# All platforms at once
./build.sh

# Or manually:
cd go-library
GOOS=windows GOARCH=amd64 go build -o network-library.exe main.go
GOOS=linux GOARCH=amd64 go build -o network-library main.go
```

### Run Server
```bash
# Linux/Mac
./build/network-library-linux-amd64

# Windows
network-library-windows-amd64.exe

# You'll see:
#   Listening on port 8270
#   Ready for Robot Framework connections
```

### Run Tests
```bash
cd robot-tests

# All tests (10 test cases)
robot testcases/poc_test.robot

# One test only
robot --test "TEST-001*" testcases/poc_test.robot

# By tag
robot --include connection testcases/poc_test.robot
robot --include routing testcases/poc_test.robot
robot --include core testcases/poc_test.robot

# With debug output
robot --loglevel DEBUG testcases/poc_test.robot
```

---

## 🔧 Configuration Checklist

### Before First Run:

1. **Update credentials** in `robot-tests/data/devices.yaml`:
```yaml
credentials:
  username: "admin"      # ← Your username
  password: "admin"      # ← Your password
```

2. **Test manual SSH**:
```bash
ssh admin@172.10.1.1
# Make sure you can connect before running tests
```

3. **Verify Go installed**:
```bash
go version
# Should show: go version go1.21.x
```

4. **Verify Robot installed**:
```bash
robot --version
# Should show: Robot Framework 7.0
# If not: pip install robotframework
```

---

## 🎯 Your Lab Devices (from host_info.csv)

### Core Routers (ASR9906):
- UPE1:  172.10.1.1
- UPE9:  172.10.1.9
- UPE21: 172.10.1.21
- UPE24: 172.10.1.24

### Aggregation (ASR903):
- SR201: 172.10.1.201
- SR202: 172.10.1.202
- SR203: 172.10.1.203
- SR204: 172.10.1.204

### Switch (Cat9300):
- Switch1: 172.10.1.50

---

## 🔍 Troubleshooting Fast Track

### Server won't start
```bash
# Check if port is in use
netstat -an | grep 8270
# Kill any existing process
pkill network-library
```

### Can't connect to devices
```bash
# 1. Ping test
ping 172.10.1.1

# 2. Manual SSH test
ssh admin@172.10.1.1

# 3. Check device credentials in devices.yaml
```

### Tests fail
```bash
# Run with debug output
robot --loglevel DEBUG testcases/poc_test.robot

# Check server console for SSH errors
# Check log.html for detailed failure info
```

### Robot not found
```bash
pip install robotframework
# Or: pip install -r requirements.txt
```

---

## 📊 Available Robot Keywords

Use these in your test cases:

```robot
${handle}=    GoLib.Connect To Device    IP    TYPE    USER    PASS
${output}=    GoLib.Execute Command    ${handle}    show version
${ospf}=      GoLib.Get OSPF Neighbors    ${handle}
${bgp}=       GoLib.Get BGP Summary    ${handle}    default
${status}=    GoLib.Get Interface Status    ${handle}    Gi0/0/0/0
${ping}=      GoLib.Ping Test    ${handle}    TARGET_IP    VRF    COUNT
              GoLib.Close Connection    ${handle}
```

---

## 📈 Success Metrics

| What to Check | Expected | How to Verify |
|---------------|----------|---------------|
| Binary size | < 15MB | `ls -lh build/` |
| Startup time | < 1 sec | Time to "Ready" message |
| Connect time | < 2 sec | Check log.html |
| Memory usage | < 20MB | `top` or Task Manager |
| Test duration | < 5 min | Check report.html |

---

## 🎬 Demo Script (5 minutes)

Perfect for showing to management:

```bash
# Terminal 1: Start server
./build/network-library-linux-amd64

# Terminal 2: Run tests
cd robot-tests
robot testcases/poc_test.robot

# Wait for tests to complete...

# Show results
open report.html
```

**Key talking points:**
1. "Single binary, no dependencies"
2. "Works on Windows, Linux, Mac"
3. "Tests are human-readable"
4. "5-10x faster than Python"
5. "Professional HTML reports"

---

## 💡 Pro Tips

### Tip 1: Run specific devices
Edit `poc_test.robot` to test only your devices:
```robot
@{CORE_DEVICES}=    Create List
...    172.10.1.1    # UPE1 - Test this one first
```

### Tip 2: Parallel testing
```bash
# Install pabot
pip install robotframework-pabot

# Run tests in parallel
pabot --processes 4 testcases/*.robot
```

### Tip 3: Quick smoke test
```bash
# Test just the connection
robot --include connection testcases/poc_test.robot
# Takes 30 seconds instead of 5 minutes
```

### Tip 4: Create your own keywords
```robot
*** Keywords ***
Quick Health Check
    [Arguments]    ${device_ip}
    ${handle}=    Connect And Validate    ${device_ip}
    ${ospf}=    GoLib.Get OSPF Neighbors    ${handle}
    ${bgp}=     GoLib.Get BGP Summary    ${handle}    default
    GoLib.Close Connection    ${handle}
```

---

## 🚦 Traffic Light Status

### 🟢 GREEN - Ready to Proceed
- Go binary compiled successfully
- Can connect to at least one device
- Basic tests passing
- Reports generated

### 🟡 YELLOW - Needs Attention
- Some tests failing (but not critical)
- Slow performance (> 10 sec per test)
- Manual SSH works but Go library doesn't

### 🔴 RED - Stop and Fix
- Can't build Go binary
- Can't connect to any devices
- Server crashes or won't start
- All tests failing

---

## 📞 Quick Help

**Read this first:** README.md  
**Step-by-step guide:** LAB_TESTING_GUIDE.md  
**Architecture details:** PROJECT_STRUCTURE.md  

**Most common fix:**
```bash
# Update credentials
vim robot-tests/data/devices.yaml

# Restart server
pkill network-library
./build/network-library-linux-amd64

# Re-run tests
robot testcases/poc_test.robot
```

---

## ✅ POC Success Checklist

- [ ] Go binary runs (see "Listening on port 8270")
- [ ] Can connect to UPE1 (172.10.1.1)
- [ ] Can execute show version
- [ ] OSPF neighbors retrieved
- [ ] BGP summary retrieved
- [ ] Ping test works
- [ ] report.html generated
- [ ] log.html shows PASS statuses
- [ ] No Python SSH packages installed

**When all checked: POC is successful! 🎉**

---

## 🎯 Next Steps After POC

1. **Demo to stakeholders** (15 min - use demo script)
2. **Get approval** for production development
3. **Expand test coverage** (VRFs, MPLS SR, interfaces)
4. **Field engineer training** (2 hours)
5. **Production migration** (use advanced_migration.robot)

---

**Remember: One binary, no dependencies, just works!** 🚀
