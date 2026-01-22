# MERALCO Network Health Check Logger

## Overview

A multi-threaded SSH command collection tool designed for the ASR9010 to ASR9906 core network migration project. This tool automates the collection of show commands from multiple network devices in parallel, generating timestamped logs for pre-migration validation, post-migration verification, and ongoing health checks.

## Features

- **Multi-threaded Execution**: Parallel SSH connections for fast data collection
- **Excel-based Inventory**: Device information stored in `host_info.xlsx`
- **Flexible Targeting**: Specify devices via `target.txt`
- **Customizable Commands**: Define commands in `command.txt`
- **Timestamped Output**: Organized logs with device-specific files
- **Migration Phase Support**: Pre-migration, post-migration, and health-check modes
- **Cross-Platform**: Single binary runs on Linux (Ubuntu, CentOS, RHEL)

## Directory Structure

```
health_check_logger/
├── ssh_health_check          # Main executable
├── ssh_nested_multi_thread.go # Source code
├── command.txt               # Show commands to execute
├── target.txt                # Target device list
├── host_info.xlsx            # Device inventory (hostname, IP, type, site, role)
└── output/                   # Generated output directory
    └── <phase>/
        └── <timestamp>/
            ├── PDLPASR9K_<timestamp>.log
            ├── DHTPASR9K_<timestamp>.log
            └── SUMMARY_<timestamp>.log
```

## Quick Start

### Prerequisites

```bash
# Install sshpass (required for non-interactive SSH)
# Ubuntu/Debian
sudo apt-get install sshpass

# CentOS/RHEL
sudo yum install sshpass

# macOS (via Homebrew)
brew install hudochenkov/sshpass/sshpass
```

### Basic Usage

```bash
# Run health check with default settings
./ssh_health_check -u admin -p YourPassword

# Verbose output with specific phase
./ssh_health_check -u admin -p YourPassword -v -phase pre-migration

# Dry run (test without connecting)
./ssh_health_check -u admin -p YourPassword -dry-run

# Custom files and workers
./ssh_health_check -u admin -p YourPassword \
    -c my_commands.txt \
    -t my_targets.txt \
    -h my_inventory.xlsx \
    -w 10
```

### Command Line Options

| Option | Short | Description | Default |
|--------|-------|-------------|---------|
| `-username` | `-u` | SSH username (required) | - |
| `-password` | `-p` | SSH password (required) | - |
| `-commands` | `-c` | Command file path | `command.txt` |
| `-targets` | `-t` | Target devices file | `target.txt` |
| `-hosts` | `-h` | Host inventory Excel file | `host_info.xlsx` |
| `-output` | `-o` | Output directory | `output` |
| `-workers` | `-w` | Max concurrent workers | 5 |
| `-port` | - | SSH port | 22 |
| `-phase` | - | Migration phase | `health-check` |
| `-verbose` | `-v` | Enable verbose output | false |
| `-dry-run` | - | Test without connecting | false |
| `-ssh-timeout` | - | SSH timeout (seconds) | 30 |
| `-cmd-timeout` | - | Command timeout (seconds) | 60 |

## File Formats

### command.txt

```
# Comments start with # or //
# One command per line
show version
show platform
show interfaces brief
show ospf neighbor
show bgp summary
show mpls ldp neighbor
show vrf all
show running-config
```

### target.txt

```
# Device hostnames (must match host_info.xlsx)
PDLPASR9K
DHTPASR9K
PDLASR01
DHTASR01
# Uncomment for aggregation devices
#BLGTRPTPE01
#MLLSUBPE01
```

### host_info.xlsx

| Hostname | IP_Address | Device_Type | Site | Role |
|----------|------------|-------------|------|------|
| PDLPASR9K | 10.228.201.9 | ASR9906 | Pasig DC | Core P Router (New) |
| DHTPASR9K | 10.228.201.13 | ASR9906 | Duhat DC | Core P Router (New) |
| PDLASR01 | 10.228.0.3 | ASR9010 | Pasig DC | Core P Router (Legacy) |
| DHTASR01 | 10.228.0.4 | ASR9010 | Duhat DC | Core P Router (Legacy) |

## Migration Workflow

### Pre-Migration Health Check (T-1 Day)

```bash
# Collect baseline from all devices
./ssh_health_check -u admin -p Password123 \
    -phase pre-migration \
    -v \
    -w 5
```

### Post-Migration Validation (T+0)

```bash
# Verify services after migration
./ssh_health_check -u admin -p Password123 \
    -phase post-migration \
    -c command_post.txt \
    -t target_new_core.txt \
    -v
```

### Ongoing Health Monitoring

```bash
# Regular health checks
./ssh_health_check -u admin -p Password123 \
    -phase health-check \
    -c command_quick.txt \
    -w 10
```

## Output Examples

### Device Log (PDLPASR9K_20260119_150000.log)

```
================================================================================
 MERALCO Network Health Check Logger
 Phase: pre-migration
================================================================================
 Hostname:    PDLPASR9K
 IP Address:  10.228.201.9
 Device Type: ASR9906
 Site:        Pasig Data Center
 Role:        Core P Router (New)
 Timestamp:   2026-01-19 15:00:00 PST
================================================================================

--------------------------------------------------------------------------------
 Command: show version
 Duration: 2.5s
--------------------------------------------------------------------------------
Cisco IOS XR Software, Version 7.9.2
Copyright (c) 2013-2024 by Cisco Systems, Inc.
...

--------------------------------------------------------------------------------
 Command: show bgp summary
 Duration: 1.8s
--------------------------------------------------------------------------------
BGP router identifier 10.228.201.9, local AS number 65000
...
```

### Summary Log (SUMMARY_20260119_150000.log)

```
================================================================================
 MERALCO Network Health Check - Execution Summary
 Phase: pre-migration
 Timestamp: 2026-01-19 15:05:00 PST
================================================================================

EXECUTION STATISTICS:
  Total Devices:    4
  Successful:       4
  Failed:           0
  Success Rate:     100.0%

DEVICE STATUS:
--------------------------------------------------------------------------------
HOSTNAME             IP ADDRESS      STATUS     NOTES
--------------------------------------------------------------------------------
PDLPASR9K            10.228.201.9    SUCCESS    
DHTPASR9K            10.228.201.13   SUCCESS    
PDLASR01             10.228.0.3      SUCCESS    
DHTASR01             10.228.0.4      SUCCESS    
--------------------------------------------------------------------------------
```

## Building from Source

### Requirements
- Go 1.22 or later
- sshpass installed on runtime system

### Build Commands

```bash
# Build for current platform
go build -o ssh_health_check ssh_nested_multi_thread.go

# Build for Linux (amd64)
GOOS=linux GOARCH=amd64 go build -o ssh_health_check_linux_amd64 ssh_nested_multi_thread.go

# Build for Linux (arm64)
GOOS=linux GOARCH=arm64 go build -o ssh_health_check_linux_arm64 ssh_nested_multi_thread.go

# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o ssh_health_check_darwin ssh_nested_multi_thread.go

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o ssh_health_check.exe ssh_nested_multi_thread.go
```

## Integration with Migration MOP

This tool supports the migration workflow defined in the MERALCO Core Migration MOP:

1. **Pre-Migration Baseline (Step 2-4 of MOP)**
   - Capture interface, routing, and system status
   - Verify IGP and BGP states
   - Backup configurations

2. **During Migration Monitoring**
   - Quick health checks between steps
   - Verify routing convergence

3. **Post-Migration Validation (Step 25-29 of MOP)**
   - Verify all services restored
   - Check interface counters/errors
   - Validate VRF route counts

## Troubleshooting

### Connection Issues

```bash
# Test SSH connectivity manually
sshpass -p 'Password' ssh -o StrictHostKeyChecking=no admin@10.228.201.9

# Check if sshpass is installed
which sshpass
```

### Permission Denied

```bash
# Make executable
chmod +x ssh_health_check
```

### Device Not Found

- Verify hostname in `target.txt` matches exactly with `host_info.xlsx`
- Hostnames are case-insensitive

### Timeout Issues

```bash
# Increase timeouts for slow devices
./ssh_health_check -u admin -p Password \
    -ssh-timeout 60 \
    -cmd-timeout 120
```

## Security Notes

- Passwords are passed via command line (visible in process list)
- For production use, consider:
  - Using SSH key authentication
  - Implementing credential vault integration
  - Running in a secure environment

## Author

Network Engineering Team  
MERALCO Core Migration Project  
Version 1.0.0

## License

Internal Use Only - MERALCO Network Infrastructure
