# MERALCO Core Network Migration - Automated Testing Suite
## Implementation Guide for Robot Framework + Go Automation

---

## 📋 Overview

This comprehensive testing suite automates the validation for MERALCO's ASR9010 → ASR9906 core network migration. It covers all critical phases:

1. **Pre-Migration Baseline Capture**
2. **During-Migration Real-Time Monitoring**
3. **Post-Migration Validation**
4. **VRF-Specific Service Testing** (all 20+ VRFs)

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│         Robot Framework Test Suites                     │
│  ┌─────────────┐ ┌──────────────┐ ┌────────────────┐  │
│  │ Pre-Mig     │ │ During-Mig   │ │ Post-Mig       │  │
│  │ Baseline    │ │ Monitoring   │ │ Validation     │  │
│  └──────┬──────┘ └──────┬───────┘ └────────┬───────┘  │
│         │                │                   │          │
│         └────────────────┼───────────────────┘          │
│                          │                              │
│                ┌─────────▼─────────┐                    │
│                │ GoNetworkLibrary  │                    │
│                │  (Custom Python)  │                    │
│                └─────────┬─────────┘                    │
└──────────────────────────┼──────────────────────────────┘
                           │
                ┌──────────▼──────────┐
                │   Go Server (8270)  │
                │  Interactive SSH    │
                └──────────┬──────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
    ┌─────▼──────┐  ┌──────▼──────┐  ┌─────▼──────┐
    │  ASR9906   │  │   ASR903    │  │  ASR9010   │
    │   (UPE1)   │  │  (SR201/2)  │  │  (Legacy)  │
    └────────────┘  └─────────────┘  └────────────┘
```

---

## 📦 Files Delivered

### Test Suites
1. **test_pre_migration_baseline.robot** - Pre-migration baseline capture
   - Device inventory
   - OSPF/BGP/MPLS state
   - Interface status
   - Connectivity matrix
   - Performance baseline
   - Go/No-Go report generation

2. **test_during_migration_monitoring.robot** - Real-time monitoring
   - Continuous OSPF/BGP monitoring
   - SCADA connectivity (5-sec intervals)
   - Teleprotection latency monitoring
   - Automatic rollback triggers
   - Real-time status dashboard

3. **test_vrf_validation.robot** - VRF-specific validation
   - All 20 VRFs tested
   - Priority-based testing strategy
   - SCADA detailed validation
   - Teleprotection latency verification
   - ADMS connectivity checks
   - Validation matrix generation

### Configuration Files
4. **devices.yaml** - Device inventory
   - Core routers (ASR9906)
   - Aggregation routers (ASR903)
   - Legacy devices (ASR9010)
   - Critical links
   - Rollback links
   - Management endpoints

5. **meralco_vrfs.yaml** - VRF configuration
   - All 20 VRFs with RD/RT
   - Priority levels (Critical/High/Medium/Low)
   - SLA requirements per VRF
   - Test endpoints
   - Testing strategy per priority

---

## 🚀 Installation & Setup

### Prerequisites
```bash
# 1. Go server (already built from POC)
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc

# 2. Python dependencies
pip3 install robotframework pyyaml

# 3. Custom library (from POC)
# GoNetworkLibrary.py already created
```

### Directory Structure
```
migration-poc/
├── go-library/
│   ├── main.go                          # Interactive SSH implementation
│   └── ... (Go dependencies)
├── build/
│   └── network-library-linux-amd64      # Compiled Go binary
├── robot-tests/
│   ├── testcases/
│   │   ├── test_pre_migration_baseline.robot
│   │   ├── test_during_migration_monitoring.robot
│   │   └── test_vrf_validation.robot
│   ├── data/
│   │   ├── devices.yaml
│   │   └── meralco_vrfs.yaml
│   ├── libraries/
│   │   └── GoNetworkLibrary.py
│   ├── baseline/                        # Pre-migration captures
│   └── reports/                         # Test reports
├── server.sh                            # Server management
└── run-tests.sh                         # Test runner
```

### Setup Steps

```bash
# 1. Create directory structure
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc/robot-tests
mkdir -p testcases data libraries baseline reports

# 2. Copy test files
cp test_pre_migration_baseline.robot testcases/
cp test_during_migration_monitoring.robot testcases/
cp test_vrf_validation.robot testcases/

# 3. Copy configuration files
cp devices.yaml data/
cp meralco_vrfs.yaml data/

# 4. Copy library (already exists from POC)
# GoNetworkLibrary.py is already in place

# 5. Update devices.yaml with your actual IPs
nano data/devices.yaml
# Edit IP addresses to match your lab

# 6. Update meralco_vrfs.yaml with your test endpoints
nano data/meralco_vrfs.yaml
# Edit test endpoints to match your lab
```

---

## 🧪 Running the Tests

### 1. Start Go Server
```bash
cd /home/cisco/Pre_Post/network-automation-lab/migration-poc
./server.sh start

# Verify server is running
./server.sh status
```

### 2. Pre-Migration Baseline Capture
```bash
cd robot-tests

# Run full baseline capture
robot --pythonpath libraries \
      --outputdir baseline \
      --name "Pre-Migration Baseline" \
      testcases/test_pre_migration_baseline.robot

# View results
firefox baseline/report.html
```

### 3. During Migration Monitoring (Run during actual cutover)
```bash
# Terminal 1: Start continuous monitoring
robot --pythonpath libraries \
      --outputdir reports \
      --name "Migration Monitoring" \
      testcases/test_during_migration_monitoring.robot

# Terminal 2: Watch real-time dashboard
tail -f reports/log.html
```

### 4. Post-Migration Validation
```bash
# After migration complete, run VRF validation
robot --pythonpath libraries \
      --outputdir reports \
      --name "VRF Validation" \
      --variable VALIDATION_MODE:post \
      testcases/test_vrf_validation.robot
```

### 5. Run All Tests in Sequence
```bash
# Create test runner script
cat > run-all-tests.sh << 'EOF'
#!/bin/bash

echo "Starting MERALCO Migration Test Suite..."

# Start server
../server.sh start
sleep 3

# Pre-migration baseline
echo "=== Running Pre-Migration Baseline ==="
robot --pythonpath libraries \
      --outputdir baseline \
      testcases/test_pre_migration_baseline.robot

# VRF validation
echo "=== Running VRF Validation ==="
robot --pythonpath libraries \
      --outputdir reports \
      --variable VALIDATION_MODE:pre \
      testcases/test_vrf_validation.robot

echo "=== Tests Complete ==="
echo "Baseline reports: baseline/"
echo "VRF reports: reports/"
EOF

chmod +x run-all-tests.sh
./run-all-tests.sh
```

---

## 📊 Test Coverage

### Pre-Migration Baseline (PRE-001 to PRE-014)
- ✅ Device inventory and software versions
- ✅ OSPF neighbor state (all routers)
- ✅ BGP sessions (all VRFs)
- ✅ Interface status and error counters
- ✅ Connectivity matrix (critical services)
- ✅ MPLS forwarding state
- ✅ Route counts per VRF
- ✅ Performance baseline (latency/jitter)
- ✅ Service-specific validation checkpoints
- ✅ Go/No-Go decision report

### During Migration Monitoring (MONITOR-001 to MONITOR-007)
- ✅ Continuous OSPF convergence (5-sec intervals)
- ✅ Continuous BGP session state (5-sec intervals)
- ✅ SCADA connectivity monitoring (5-sec intervals)
- ✅ Teleprotection latency monitoring (< 10ms required)
- ✅ Interface status monitoring (10-sec intervals)
- ✅ MPLS label distribution (30-sec intervals)
- ✅ Real-time status dashboard (15-sec updates)
- ✅ Automatic rollback triggers

### VRF Validation (VRF-001 to VRF-007)
- ✅ Critical VRFs comprehensive suite
- ✅ High priority VRFs standard suite
- ✅ Medium/Low VRFs basic suite
- ✅ VPN_SCADA detailed validation
- ✅ VPN_Telepro latency verification (< 10ms)
- ✅ VPN_ADMS connectivity validation
- ✅ Validation matrix generation

---

## 🎯 Critical Success Criteria

### SCADA (VPN_SCADA) - CRITICAL
- ✅ BGP sessions established
- ✅ 100% ping success required
- ✅ Average latency < 200ms
- ✅ RTU polling successful

### Teleprotection (VPN_Telepro) - CRITICAL
- ✅ BGP sessions established
- ✅ 100% ping success required
- ✅ **Average latency < 10ms (HARD LIMIT)**
- ✅ Maximum latency < 15ms
- ✅ Jitter < 2ms

### ADMS (VPN_ADMS) - CRITICAL
- ✅ BGP sessions established
- ✅ ≥ 95% ping success
- ✅ Average latency < 100ms
- ✅ ADMS servers reachable

---

## 🔄 Rollback Triggers

### Automatic Rollback Conditions
1. **OSPF not converged** after 15 seconds
2. **BGP sessions down** for > 120 seconds
3. **SCADA connectivity lost** for > 60 seconds
4. **Teleprotection latency > 10ms** for > 30 seconds
5. **Any critical VRF fails** validation
6. **> 50% of all VRFs fail** validation

---

## 📈 Reporting

### Generated Reports
1. **Baseline Report** (`baseline/report.html`)
   - Complete pre-migration state
   - Go/No-Go recommendation
   - Configuration snapshots
   - Performance metrics

2. **Monitoring Dashboard** (`reports/log.html`)
   - Real-time status updates
   - Downtime counters
   - Rollback trigger status
   - Service health indicators

3. **VRF Validation Matrix** (`reports/report.html`)
   - Per-VRF test results
   - Pass/fail statistics
   - Failed VRF list
   - Rollback recommendation

---

## 🔧 Customization for Your Lab

### 1. Update Device IPs
Edit `data/devices.yaml`:
```yaml
core_devices:
  - hostname: UPE1
    ip: 172.10.1.1        # <-- Change to your UPE1 IP
    device_type: ASR9906
```

### 2. Update Test Endpoints
Edit `data/meralco_vrfs.yaml`:
```yaml
vrfs:
  - name: VPN_SCADA
    test_endpoints:
      - ip: 10.240.1.1    # <-- Change to your SCADA server
        description: "HQ SCADA Master"
```

### 3. Adjust Credentials
In test files, update:
```robot
${USERNAME}    admin      # <-- Your username
${PASSWORD}    admin      # <-- Your password
```

### 4. Customize Thresholds
In test files, adjust thresholds:
```robot
${SCADA_OUTAGE_LIMIT}     60    # seconds
${TELEPRO_LATENCY_LIMIT}  10    # milliseconds
```

---

## 🚨 Troubleshooting

### Server Not Starting
```bash
# Check if port 8270 is already in use
netstat -tulpn | grep 8270

# Kill existing process
./server.sh stop

# Check server log
./server.sh log
```

### Tests Failing to Connect
```bash
# Verify devices are reachable
ping 172.10.1.1

# Test SSH manually
ssh admin@172.10.1.1

# Check firewall rules
iptables -L
```

### Robot Framework Errors
```bash
# Verify Python path
robot --pythonpath libraries --version

# Check if library loads
python3 -c "import sys; sys.path.append('libraries'); import GoNetworkLibrary"

# Run with debug output
robot --pythonpath libraries --loglevel DEBUG testcases/test_pre_migration_baseline.robot
```

---

## 📝 Next Steps

1. **Adapt to Your Lab**
   - Update IP addresses in YAML files
   - Configure test endpoints
   - Adjust credentials

2. **Run Baseline Capture**
   - Execute pre-migration tests
   - Review Go/No-Go report
   - Store baseline for comparison

3. **Practice Migration**
   - Run monitoring suite in test environment
   - Validate rollback triggers work
   - Measure actual timing

4. **Production Execution**
   - Run baseline 24 hours before MW
   - Start monitoring at T+0
   - Execute VRF validation post-migration

---

## 📚 Reference

### Key Files
- `main-final-working.go` - Interactive SSH implementation
- `GoNetworkLibrary.py` - Custom Robot Framework library
- `devices.yaml` - Infrastructure inventory
- `meralco_vrfs.yaml` - VRF configuration

### MERALCO Migration Context
- **Source**: ASR9010 (Legacy Core)
- **Target**: ASR9906 (New Core with MPLS-SR)
- **Critical Services**: SCADA, Teleprotection, ADMS
- **Grid**: Plaridel-Duhat (Pilot)
- **Downtime Window**: 2 hours

---

## ✅ Success Checklist

- [ ] Go server running on port 8270
- [ ] Device IPs updated in devices.yaml
- [ ] Test endpoints configured in meralco_vrfs.yaml
- [ ] Pre-migration baseline captured
- [ ] Baseline reports reviewed
- [ ] Go/No-Go decision made
- [ ] Monitoring suite tested
- [ ] Rollback procedures validated
- [ ] VRF validation suite verified
- [ ] Reports accessible and readable

---

**Created for MERALCO Core Network Migration**  
**ASR9010 → ASR9906 with MPLS Segment Routing**  
**Powered by Go + Robot Framework Automation** 🚀
