# Lab Testing Guide - Step by Step

## Prerequisites Checklist

### On Your Development Machine:
- [ ] Go 1.21+ installed (`go version`)
- [ ] Git installed (for cloning project)
- [ ] Network connectivity to lab devices

### On Lab Test Machine:
- [ ] Network connectivity to lab devices (172.10.1.x)
- [ ] SSH access to devices enabled
- [ ] Device credentials available
- [ ] Python 3.8+ (only if running Robot Framework tests)

### Lab Network Requirements:
- [ ] Can ping 172.10.1.1 (UPE1)
- [ ] Can ping 172.10.1.201 (SR201)
- [ ] SSH port 22 accessible
- [ ] No firewall blocking connections

---

## Setup Process

### Phase 1: Build the Go Library (5 minutes)

#### On Linux/macOS:
```bash
cd migration-poc
./build.sh
```

#### On Windows:
```cmd
cd migration-poc
build.bat
```

**Expected Output:**
```
========================================
Building Network Migration Go Library
Version: 1.0.0-poc
========================================

→ Downloading dependencies...
→ Building binaries...
  • Windows (amd64)...
  • Linux (amd64)...
  • Linux (arm64)...
  • macOS (amd64)...
  • macOS (arm64)...

✓ Build completed successfully!
```

**Verify Build:**
```bash
ls -lh build/
# Should show 5 binary files (~10MB each)
```

---

### Phase 2: Configure Lab Credentials (2 minutes)

Edit `robot-tests/data/devices.yaml`:

```yaml
credentials:
  username: "your_username"    # ← Change this
  password: "your_password"    # ← Change this
```

**Test Manual SSH Connection First:**
```bash
# Make sure you can SSH manually before testing
ssh admin@172.10.1.1
show version
exit
```

---

### Phase 3: Start the Go Library Server (1 minute)

#### On Linux:
```bash
cd build
chmod +x network-library-linux-amd64
./network-library-linux-amd64
```

#### On Windows:
```cmd
cd build
network-library-windows-amd64.exe
```

#### On macOS:
```bash
cd build
chmod +x network-library-darwin-amd64
./network-library-darwin-amd64
```

**Expected Output:**
```
===============================================================================
  Network Migration Go Remote Library
  Listening on port 8270
  Ready for Robot Framework connections
===============================================================================
```

**Verify Server is Running:**
```bash
# In another terminal
netstat -an | grep 8270
# Should show: tcp 0.0.0.0:8270 LISTEN
```

---

### Phase 4: Install Robot Framework (3 minutes)

#### Option 1: Using pip
```bash
pip install robotframework
```

#### Option 2: Using requirements.txt
```bash
cd migration-poc
pip install -r requirements.txt
```

**Verify Installation:**
```bash
robot --version
# Should show: Robot Framework 7.0 (Python 3.x on linux)
```

---

### Phase 5: Run POC Tests (2 minutes)

#### Quick Smoke Test:
```bash
cd robot-tests
robot --test "TEST-001*" testcases/poc_test.robot
```

This runs only the first test (library connectivity check).

**Expected Output:**
```
==============================================================================
Poc Test
==============================================================================
TEST-001: Verify Go Remote Library Connection                       | PASS |
------------------------------------------------------------------------------
Poc Test                                                             | PASS |
1 test, 1 passed, 0 failed
==============================================================================
```

#### Full POC Test Suite:
```bash
cd robot-tests
robot testcases/poc_test.robot
```

This runs all 10 test cases.

#### Run Specific Tests by Tags:
```bash
# Run only connection tests
robot --include connection testcases/poc_test.robot

# Run only routing tests
robot --include routing testcases/poc_test.robot

# Run only core router tests
robot --include core testcases/poc_test.robot
```

---

### Phase 6: View Test Results

Robot Framework automatically generates reports:

```bash
cd robot-tests
# View reports (auto-generated in current directory)
open report.html    # macOS
xdg-open report.html    # Linux
start report.html   # Windows
```

**Three Reports Generated:**
1. **report.html** - Executive summary with statistics
2. **log.html** - Detailed test execution log
3. **output.xml** - Machine-readable results

---

## Troubleshooting Common Issues

### Issue 1: Build Fails

**Error:** `go: command not found`

**Solution:**
```bash
# Install Go
# Linux:
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# macOS:
brew install go

# Windows:
# Download installer from https://go.dev/dl/
```

### Issue 2: Server Won't Start

**Error:** `bind: address already in use`

**Solution:**
```bash
# Find what's using port 8270
lsof -i :8270
# Or on Windows:
netstat -ano | findstr :8270

# Kill the process or use different port
# Edit main.go: change ":8270" to ":8271"
```

### Issue 3: Can't Connect to Devices

**Error:** `SSH connection failed: connection timeout`

**Check:**
```bash
# 1. Can you ping the device?
ping 172.10.1.1

# 2. Can you SSH manually?
ssh admin@172.10.1.1

# 3. Is SSH enabled on device?
# On device:
show ssh

# 4. Check firewall
# Windows: Windows Defender Firewall
# Linux: sudo iptables -L
```

### Issue 4: Authentication Failure

**Error:** `SSH connection failed: authentication failed`

**Solution:**
1. Verify credentials in `devices.yaml`
2. Try SSH manually with same credentials
3. Check if account is locked
4. Verify TACACS/RADIUS is working

### Issue 5: Robot Framework Not Found

**Error:** `robot: command not found`

**Solution:**
```bash
# Install Robot Framework
pip install robotframework

# If pip not found, install Python first
# Then retry pip install
```

### Issue 6: Tests Timeout

**Error:** Tests hang or timeout

**Solution:**
```bash
# Increase timeout in test
# Edit poc_test.robot:
# Add: [Timeout]    5 minutes

# Or run with verbose logging
robot --loglevel DEBUG testcases/poc_test.robot
```

---

## Success Criteria for Lab POC

Your POC is successful when you can demonstrate:

### Minimum Viable POC (30 minutes):
- [ ] Go binary compiled and running
- [ ] Robot Framework installed
- [ ] Connected to at least one device (UPE1 or SR201)
- [ ] Executed one command successfully
- [ ] Generated HTML test report

### Complete POC (2 hours):
- [ ] All 10 POC tests passing
- [ ] Connected to both ASR9906 and ASR903
- [ ] Retrieved OSPF neighbor data
- [ ] Retrieved BGP summary data
- [ ] Ping tests working
- [ ] HTML reports generated
- [ ] No Python SSH libraries installed (verified)

### Production-Ready POC (1 day):
- [ ] Advanced migration tests working
- [ ] Baseline capture functional
- [ ] Post-migration comparison working
- [ ] Multiple devices tested in parallel
- [ ] Field engineer can run without assistance
- [ ] Documentation complete

---

## Quick Commands Reference

### Build:
```bash
./build.sh                  # Build all platforms
```

### Run Server:
```bash
./build/network-library-linux-amd64
```

### Run Tests:
```bash
# All tests
robot testcases/poc_test.robot

# Specific test
robot --test "TEST-001*" testcases/poc_test.robot

# By tag
robot --include critical testcases/poc_test.robot

# With detailed logging
robot --loglevel DEBUG testcases/poc_test.robot
```

### View Reports:
```bash
open report.html
open log.html
```

### Stop Server:
```bash
# Press Ctrl+C in server terminal
# Or:
pkill network-library
```

---

## Performance Benchmarks to Measure

During your POC, measure these metrics:

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Server startup time | < 1 second | Time from run to "Ready" message |
| Connect to device | < 2 seconds | Check log.html timestamps |
| Execute command | < 1 second | Check log.html timestamps |
| Parse OSPF output | < 0.1 seconds | Check log.html timestamps |
| Total test suite | < 5 minutes | Check report.html duration |
| Memory usage | < 20MB | `top` or Task Manager |
| Binary size | < 15MB | `ls -lh build/` |

---

## Next Steps After Successful POC

### Immediate (Week 1):
1. Demo to Change Management Board
2. Show HTML reports to stakeholders
3. Get approval for production development

### Short-term (Week 2-3):
1. Add VRF-specific validation
2. Implement MPLS SR checks
3. Create pre/post migration suites
4. Add Excel report generation

### Long-term (Month 1-2):
1. CI/CD pipeline integration
2. Scheduled health checks
3. Migration automation framework
4. Field engineer training program

---

## Support Resources

**During Lab Testing:**
- Check README.md for detailed documentation
- Review log.html for test execution details
- Check Go library console for SSH connection logs
- Use `--loglevel DEBUG` for maximum detail

**Common Commands:**
```bash
# Check Go version
go version

# Check Robot version
robot --version

# Check network connectivity
ping 172.10.1.1
telnet 172.10.1.1 22

# Check if server is running
netstat -an | grep 8270
ps aux | grep network-library

# Check Python packages
pip list | grep robot
```

---

## POC Demo Script (15 minutes)

Use this script to demo the POC to stakeholders:

### 1. Introduction (2 min)
- "This POC shows Go binary + Robot Framework"
- "Zero Python dependencies on field laptops"
- "Same tests work on Windows, Linux, Mac"

### 2. Show Binary Deployment (2 min)
```bash
ls -lh build/
# Point out: just 10MB, no dependencies
```

### 3. Start Server (1 min)
```bash
./build/network-library-linux-amd64
# Point out: instant startup, ready immediately
```

### 4. Show Test Cases (3 min)
```bash
cat robot-tests/testcases/poc_test.robot
# Point out: readable, non-technical staff can understand
```

### 5. Run Tests (5 min)
```bash
robot testcases/poc_test.robot
# Watch tests execute in real-time
```

### 6. Show Results (2 min)
```bash
open report.html
# Navigate through: summary, failed/passed, detailed logs
```

### 7. Q&A
Common questions:
- "How do we update tests?" → Edit .robot files
- "How do we deploy?" → Copy one binary
- "What if Python breaks?" → Doesn't matter, Go binary independent
- "How fast is it?" → Show timestamps in log.html (5-10x faster)

---

**Ready to start testing? Begin with Phase 1!** 🚀
