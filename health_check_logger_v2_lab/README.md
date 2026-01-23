# MERALCO Network Health Check Logger v2.0

## What's New in v2.0

| Feature | v1.0 | v2.0 |
|---------|------|------|
| SSH Sessions | Multiple (1 per command) | **Single session per device** |
| Login Events | Many (per command) | **One login/logout per device** |
| PTY Allocation | Basic | **Forced (-t -t) for IOS-XR** |
| Command Files | Single file | **OS-specific files** |
| Device Detection | Manual | **Auto-detect from Device_Type** |

## Key Improvements

### 1. Single SSH Session Per Device
- **Before**: Each command = new SSH connection = new login log entry
- **After**: One connection, all commands, one logout
- **Benefit**: Cleaner device logs, faster execution

### 2. Proper PTY Allocation for IOS-XR
- Fixed "Pseudo-terminal will not be allocated" error
- Uses `-t -t` flag to force TTY allocation
- Works correctly with IOS-XRv and ASR9K devices

### 3. OS-Specific Command Files
Automatically selects the right commands based on `Device_Type` in host_info.xlsx:

| Device Type Contains | OS Detected | Command File |
|---------------------|-------------|--------------|
| ASR9, XRV, NCS, IOS-XR | IOS-XR | command_iosxr.txt |
| ASR903, ASR920, ISR, CSR, CAT, C9 | IOS-XE | command_iosxe.txt |
| SWITCH, L2, 2960, 3750, 9300 | L2-SWITCH | command_l2switch.txt |

## Directory Structure

```
health_check_logger_v2/
├── ssh_health_check              # Main executable
├── ssh_health_check_linux_amd64  # Linux x64 binary
├── command.txt                   # Default/fallback commands
├── command_iosxr.txt            # IOS-XR specific commands
├── command_iosxe.txt            # IOS-XE specific commands
├── command_l2switch.txt         # L2 Switch specific commands
├── target.txt                   # Target device list
├── host_info.xlsx               # Device inventory
└── output/                      # Generated output
```

## Quick Start

```bash
# Basic usage (auto-detects OS, uses appropriate commands)
./ssh_health_check -u admin -p Password123

# Verbose mode
./ssh_health_check -u admin -p Password123 -v

# Specify phase
./ssh_health_check -u admin -p Password123 -phase pre-migration

# Increase timeout for slow devices
./ssh_health_check -u admin -p Password123 -cmd-timeout 300
```

## Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-u, -username` | SSH username | (required) |
| `-p, -password` | SSH password | (required) |
| `-c, -commands` | Default command file | command.txt |
| `-cmd-xr` | IOS-XR command file | command_iosxr.txt |
| `-cmd-xe` | IOS-XE command file | command_iosxe.txt |
| `-cmd-l2` | L2 Switch command file | command_l2switch.txt |
| `-t, -targets` | Target devices file | target.txt |
| `-hosts` | Host inventory file | host_info.xlsx |
| `-o, -output` | Output directory | output |
| `-w, -workers` | Parallel workers | 5 |
| `-port` | SSH port | 22 |
| `-phase` | Migration phase | health-check |
| `-cmd-timeout` | Command timeout (sec) | 180 |
| `-os-commands` | Use OS-specific files | true |
| `-v, -verbose` | Verbose output | false |
| `-dry-run` | Test without connecting | false |

## host_info.xlsx Format

| Column | Header | Description | Example |
|--------|--------|-------------|---------|
| A | Hostname | Device hostname | PDLPASR9K |
| B | IP_Address | Management IP | 10.228.201.9 |
| C | Device_Type | Platform type | ASR9906, ASR903, Cat9300 |
| D | Site | Location | Pasig DC |
| E | Role | Device role | Core P Router |

**Important**: The `Device_Type` column determines which command file is used!

## Output Example

### Device Log Header
```
================================================================================
 MERALCO Network Health Check Logger v2.0
 Phase: health-check
================================================================================
 Hostname:    SR201
 IP Address:  172.10.1.201
 Device Type: ASR903
 Device OS:   IOS-XE
 Site:        Lab
 Role:        PE Aggregation
 Timestamp:   2026-01-23 10:30:00 UTC
 Session:     Single SSH session (all commands)
================================================================================
```

### Summary Shows OS Detection
```
DEVICE STATUS:
--------------------------------------------------------------------------------
HOSTNAME        IP ADDRESS      OS         STATUS     NOTES
--------------------------------------------------------------------------------
UPE1            172.10.1.1      IOS-XR     SUCCESS    
SR201           172.10.1.201    IOS-XE     SUCCESS    
Switch1         172.10.1.50     L2-SWITCH  SUCCESS    
--------------------------------------------------------------------------------
```

## Customizing Commands

### Add New Commands for IOS-XR
Edit `command_iosxr.txt`:
```
# Add your custom IOS-XR commands
show segment-routing traffic-eng policy all
show isis segment-routing label table
```

### Add New Commands for IOS-XE
Edit `command_iosxe.txt`:
```
# Add your custom IOS-XE commands
show sdwan control connections
show platform hardware fed active qos
```

## Troubleshooting

### IOS-XR: "Pseudo-terminal will not be allocated"
This is **fixed in v2.0**. The tool now uses `-t -t` to force PTY allocation.

### Command Not Supported
If a command doesn't exist on a platform, the output will show:
```
(No output or command not supported on this platform)
```
This is normal - OS-specific commands won't work on other platforms.

### Connection Timeout
Increase the timeout:
```bash
./ssh_health_check -u admin -p Password -cmd-timeout 300
```

### Too Many Parallel Connections
Reduce workers:
```bash
./ssh_health_check -u admin -p Password -w 2
```

## Building from Source

```bash
# Build for current platform
go build -o ssh_health_check ssh_nested_multi_thread.go

# Build for Linux x64 (optimized)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ssh_health_check_linux_amd64 ssh_nested_multi_thread.go
```

## Requirements

- **sshpass** must be installed on the system running the tool
  ```bash
  # Ubuntu/Debian
  sudo apt-get install sshpass
  
  # CentOS/RHEL
  sudo yum install sshpass
  ```

## Migration Workflow

```
T-1 Day:  ./ssh_health_check -u admin -p Pass -phase pre-migration
T+0:      ./ssh_health_check -u admin -p Pass -phase post-migration
T+7 Day:  ./ssh_health_check -u admin -p Pass -phase health-check
```

---
**Version**: 2.0.0  
**Author**: Network Engineering Team  
**Project**: MERALCO Core Migration (ASR9010 → ASR9906)
