# Common Setup Errors & Solutions

## Error 1: "No module named 'yaml'" or "Failed: using YAML variable files"

**Error message you're seeing:**
```
Error in file '/home/cisco/Pre_Post/network-automation-lab/migration-poc/robot-tests/testcases/poc_test.robot' 
on line 13: Processing variable file '/home/cisco/Pre_Post/network-automation-lab/migration-poc/robot-tests/data/devices.yaml' 
failed: using YAML variable files requires PyYAML module to be installed. Typically you can install it by running 'pip install pyyaml'.
```

**Solution:**
```bash
# Install PyYAML
pip3 install pyyaml

# Or use sudo if permission denied
sudo pip3 install pyyaml

# Or install in user space
pip3 install --user pyyaml
```

**Verify installation:**
```bash
python3 -c "import yaml; print('PyYAML installed successfully')"
```

---

## Error 2: "No keyword with name 'GoLib.Get Keyword Names' found"

**Error message:**
```
No keyword with name 'GoLib.Get Keyword Names' found.
```

**Cause:** Go server is not running or not accessible on port 8270

**Solution:**
```bash
# Check if Go server is running
netstat -an | grep 8270
# Should show: tcp 0.0.0.0:8270 LISTEN

# If not running, start it:
./build/network-library-linux-amd64

# You should see:
# ===============================================================================
#   Network Migration Go Remote Library
#   Listening on port 8270
#   Ready for Robot Framework connections
# ===============================================================================
```

---

## Error 3: "Calling dynamic method 'get_keyword_names' failed: Connecting remote server at http://localhost:8270"

**Cause:** Port 8270 is blocked or Go server crashed

**Solution:**
```bash
# 1. Check if server is running
ps aux | grep network-library

# 2. Check port binding
sudo netstat -tlnp | grep 8270

# 3. Check if something else is using port 8270
lsof -i :8270

# 4. Try restarting the Go server
pkill network-library
./build/network-library-linux-amd64
```

---

## Error 4: Robot Framework not found

**Error message:**
```
robot: command not found
```

**Solution:**
```bash
# Install Robot Framework
pip3 install robotframework

# Verify installation
robot --version

# If still not found, add to PATH
export PATH=$PATH:~/.local/bin

# Or use full path
python3 -m robot testcases/poc_test.robot
```

---

## Error 5: Permission denied when installing packages

**Error message:**
```
ERROR: Could not install packages due to an EnvironmentError: [Errno 13] Permission denied
```

**Solution:**
```bash
# Option 1: Install in user space (recommended)
pip3 install --user pyyaml robotframework

# Option 2: Use sudo (not recommended)
sudo pip3 install pyyaml robotframework

# Option 3: Use virtual environment (best practice)
python3 -m venv venv
source venv/bin/activate
pip install pyyaml robotframework
```

---

## Error 6: Build fails - Go not found

**Error message:**
```
./build.sh: line 23: go: command not found
```

**Solution:**
```bash
# Install Go on Linux
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Verify
go version

# Add to .bashrc for permanent
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
```

---

## Error 7: Cannot connect to devices

**Error message in Go server console:**
```
SSH connection failed: dial tcp 172.10.1.1:22: i/o timeout
```

**Solution:**
```bash
# 1. Test network connectivity
ping 172.10.1.1

# 2. Test SSH manually
ssh admin@172.10.1.1

# 3. Check if SSH is enabled on device
# On device:
show ssh

# 4. Check firewall
sudo iptables -L
```

---

## Error 8: Authentication failed

**Error message:**
```
SSH connection failed: ssh: handshake failed: ssh: unable to authenticate
```

**Solution:**
```bash
# 1. Verify credentials in devices.yaml
cat robot-tests/data/devices.yaml

# 2. Test credentials manually
ssh admin@172.10.1.1
# Enter password manually

# 3. Update credentials in devices.yaml
vim robot-tests/data/devices.yaml
# Change:
credentials:
  username: "your_actual_username"
  password: "your_actual_password"
```

---

## Complete Setup Checklist

Run this checklist to ensure everything is ready:

```bash
# 1. Check Python
python3 --version
# Should show: Python 3.8 or higher

# 2. Check pip
pip3 --version

# 3. Install dependencies
pip3 install pyyaml robotframework

# 4. Verify installations
python3 -c "import yaml; import robot; print('All Python deps OK')"

# 5. Check Go binary exists
ls -lh build/network-library-linux-amd64
# Should show: ~10MB executable

# 6. Start Go server
./build/network-library-linux-amd64 &

# 7. Verify server is running
sleep 2
netstat -an | grep 8270
# Should show: LISTEN

# 8. Update credentials
vim robot-tests/data/devices.yaml

# 9. Test connectivity
ping -c 2 172.10.1.1

# 10. Run tests
cd robot-tests
robot testcases/poc_test.robot
```

---

## Quick Fix for Your Current Error

Based on your screenshot, here's the exact fix:

```bash
# You're currently in: /home/cisco/Pre_Post/network-automation-lab/migration-poc/robot-tests

# Step 1: Install PyYAML (this is your main issue)
pip3 install pyyaml

# Step 2: Verify Go server is still running
ps aux | grep network-library
# If not running, start it:
# cd ..
# ./build/network-library-linux-amd64 &

# Step 3: Re-run tests
robot testcases/poc_test.robot
```

---

## Automated Setup Script

Instead of manual steps, use the automated setup script:

```bash
# Go to project root
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc

# Run setup script
chmod +x setup.sh
./setup.sh

# This will:
# - Check all dependencies
# - Install missing packages
# - Verify Go server
# - Test network connectivity
```

---

## Still Having Issues?

### Enable Debug Mode

```bash
# Run with debug output
robot --loglevel DEBUG testcases/poc_test.robot

# Check Go server console for errors
# The terminal where you ran ./build/network-library-linux-amd64
```

### Check Logs

```bash
# After test run, check detailed logs
less log.html
# Or open in browser

# Check Go server output
# Look at the terminal where server is running
```

### Minimal Test

Create a minimal test to isolate the issue:

```bash
# Create test_minimal.robot
cat > test_minimal.robot << 'EOF'
*** Settings ***
Library    Remote    http://localhost:8270    WITH NAME    GoLib

*** Test Cases ***
Test Connection
    ${keywords}=    GoLib.Get Keyword Names
    Log    ${keywords}
EOF

# Run minimal test
robot test_minimal.robot
```

If this works, the issue is with the YAML file or credentials.
If this fails, the issue is with Go server or Robot Framework connection.

---

## Contact Points

If you've tried everything above:

1. Check that Go server shows: "Listening on port 8270"
2. Check that `netstat -an | grep 8270` shows LISTEN
3. Check that `pip3 list | grep -i yaml` shows PyYAML
4. Check that credentials are correct in devices.yaml
5. Check that you can ping devices: `ping 172.10.1.1`

All working? Run the test again!
