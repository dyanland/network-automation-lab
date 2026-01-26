# MERALCO Network Migration Validation Toolkit v2.0

## Overview

A comprehensive, cross-platform Go-based network validation toolkit designed for the MERALCO core network migration from legacy ASR9010 routers to new ASR9906 routers with MPLS Segment Routing.

## Features

### 🔧 Device Management
- Add/remove network devices interactively
- Import/export device configurations (JSON)
- Test SSH connectivity to all devices
- Support for IOS-XR, IOS-XE, and IOS devices

### 📊 Pre-Migration Validation
- Collect comprehensive baseline (BGP, OSPF, MPLS, LDP, VRF routes)
- Validate control plane status
- Check interface errors and CRC counts
- Verify configuration backups
- Generate Go/No-Go reports

### ⚡ During-Migration Monitoring
- Real-time traffic monitoring
- BGP convergence tracking
- OSPF adjacency monitoring
- Service health checks
- Rollback trigger detection
- Quick validation snapshots

### ✅ Post-Migration Validation
- Full post-migration checks
- Baseline comparison with variance detection
- MPLS-SR label verification
- QoS policy validation
- End-to-end connectivity testing

### 🔍 Connectivity Testing
- Single and batch ping tests
- Standard and MPLS traceroute
- VRF-aware testing

### 📏 MTU Testing
- MTU discovery with DF-bit (binary search)
- Jumbo frame validation

### 📈 Traffic Drain Monitoring
- Interface traffic monitoring
- Configurable drain thresholds
- Automatic drain completion detection

### 📄 Report Generation
- Professional HTML reports with statistics
- JSON data export
- Previous report management

## Installation

### Prerequisites
- Go 1.21 or later
- Network access to target devices

### Build from Source

```bash
# Clone or download the toolkit
cd meralco-migration-toolkit

# Download dependencies
go mod tidy

# Build for current OS
go build -o meralco-toolkit main.go

# Or build for specific platforms:
# Windows
GOOS=windows GOARCH=amd64 go build -o meralco-toolkit.exe main.go

# Linux
GOOS=linux GOARCH=amd64 go build -o meralco-toolkit-linux main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o meralco-toolkit-mac main.go
```

## Configuration

### Device Configuration (devices.json)

```json
[
  {
    "hostname": "PDLPASR9K",
    "ip_address": "10.228.201.9",
    "device_type": "ios-xr",
    "role": "core",
    "username": "admin",
    "password": "your_password",
    "ssh_port": 22
  }
]
```

**Device Types:**
- `ios-xr` - Cisco IOS-XR (ASR9K, NCS series)
- `ios-xe` - Cisco IOS-XE (ASR903, ASR920)
- `ios` - Classic Cisco IOS

**Roles:**
- `core` - Core/P routers
- `aggregation` - Aggregation/PE routers
- `access` - Access/CPE routers

### Default VRFs

The toolkit comes pre-configured with MERALCO VRFs:

| VRF Name | Priority | Description |
|----------|----------|-------------|
| VPN_ADMS | Critical | MERALCO ADMS VRF |
| VPN_SCADA | Critical | SCADA RTU VRF |
| VPN_Telepro | Critical | Teleprotection VRF |
| VPN_Tetra | High | Tetra Radio VRF |
| VPN_CCTV | High | CCTV VRF |
| VPN_Metering | High | AMI Metering VRF |
| VPN_Transport_Mgt | Medium | Transport Management |
| VPN_SCADA_SIP | High | SCADA SIP VRF |

## Usage

### Starting the Toolkit

```bash
./meralco-toolkit
```

### Menu Navigation

1. **Device Management** - Configure and test devices
2. **Pre-Migration Validation** - Run pre-migration checks
3. **During-Migration Monitoring** - Real-time monitoring
4. **Post-Migration Validation** - Post-cutover verification
5. **Connectivity Testing** - Ping and traceroute tests
6. **MTU Testing** - MTU discovery
7. **Traffic Drain Monitor** - Monitor interface drain
8. **Generate Reports** - Create HTML/JSON reports
9. **Baseline Management** - Load/view baselines

### Example Workflow

#### Pre-Migration (Day before MW)

1. Start toolkit: `./meralco-toolkit`
2. Import devices: Menu 1 → Option 5 → `devices.json`
3. Test connectivity: Menu 1 → Option 4 → 0 (all devices)
4. Run full pre-migration check: Menu 2 → Option 1
5. Review Go/No-Go report

#### During Migration

1. Start quick snapshots: Menu 3 → Option 6
2. Monitor traffic drain: Menu 7 → Option 1
3. Check rollback triggers: Menu 3 → Option 5
4. Monitor BGP convergence: Menu 3 → Option 2

#### Post-Migration

1. Run full post-check: Menu 4 → Option 1
2. Compare with baseline: Menu 4 → Option 2
3. Run E2E connectivity tests: Menu 4 → Option 5
4. Generate final report: Menu 8 → Option 1 → `final`

## Output Directory

All reports and exports are saved to `./output/`:
- `baseline_YYYYMMDD_HHMMSS.json` - Baseline captures
- `pre_migration_YYYYMMDD_HHMMSS.html` - Pre-migration reports
- `post_migration_YYYYMMDD_HHMMSS.html` - Post-migration reports
- `go_nogo_YYYYMMDD_HHMMSS.html` - Go/No-Go reports
- `export_YYYYMMDD_HHMMSS.json` - JSON data exports

## Validation Checks

### Control Plane
- OSPF neighbor state (FULL)
- BGP session status (Established)
- LDP neighbor status (Operational)

### Data Plane
- MPLS forwarding table
- Interface error counters

### Service Layer
- VRF route counts
- Critical service availability

## Rollback Triggers

The toolkit automatically detects these rollback conditions:
- Critical VRF has no routes
- No BGP sessions established
- OSPF neighbors down

## Support

For issues or enhancements, contact the Network Engineering team.

## Version History

- **v2.0.0** - Complete rewrite in Go, cross-platform support
- **v1.0.0** - Initial Python scripts

## License

Internal use only - MERALCO Network Engineering
