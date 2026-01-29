# 📖 devices.yaml - Complete Explanation Guide

## Quick Answer: Where to Put Credentials

### Method 1: Hardcoded in Test Files (Current)
```robot
*** Variables ***
${USERNAME}    admin      # <-- Change this
${PASSWORD}    admin      # <-- Change this
```

**Files to edit:**
- `test_pre_migration_baseline.robot`
- `test_during_migration_monitoring.robot`
- `test_vrf_validation.robot`

### Method 2: In devices.yaml (Better)
```yaml
credentials:
  production:
    username: admin          # <-- Change this
    password: yourpassword   # <-- Change this
```

Then update test files to read from YAML:
```robot
${credentials}=    Load YAML    data/devices.yaml
${USERNAME}=    Set Variable    ${credentials}[credentials][production][username]
${PASSWORD}=    Set Variable    ${credentials}[credentials][production][password]
```

---

## 🔍 devices.yaml Section-by-Section Explanation

### 1. CREDENTIALS Section
```yaml
credentials:
  production:
    username: admin
    password: admin
```

**What it is:** Login credentials for network devices  
**Function:** Stores username/password for SSH access  
**When needed:** Every single connection to network devices  
**Must change:** YES - use your actual credentials  

**Example:**
```yaml
credentials:
  production:
    username: meralco_admin
    password: M3ralc0!2024
```

---

### 2. CORE DEVICES Section
```yaml
core_devices:
  - hostname: UPE1
    ip: 172.10.1.1
    device_type: ASR9906
    role: core
```

**What it is:** Your NEW ASR9906 core routers with Segment Routing  
**Function:** Main backbone routers that handle MPLS-SR transport  
**When needed:**
- ✅ **Pre-migration:** Baseline their state (OSPF, BGP, MPLS)
- ✅ **During migration:** Monitor convergence in real-time
- ✅ **Post-migration:** Validate everything works on new core

**Real-world example:**
```
Your network topology:
┌─────────────────────────────────────┐
│  NEW CORE (What you're migrating TO)│
│  ┌─────────┐      ┌─────────┐      │
│  │  UPE1   │──────│  UPE2   │      │  <-- These are core_devices
│  │ ASR9906 │      │ ASR9906 │      │
│  └────┬────┘      └────┬────┘      │
│       │                │            │
└───────┼────────────────┼────────────┘
        │                │
        └────────────────┘
```

**Why you need this:**
- Tests connect to UPE1/UPE2 to check if migration successful
- Validates OSPF/BGP sessions on new core
- Checks if SR-MPLS labels are distributed

**Must change:** YES - `ip:` field must match your actual UPE1 IP address

---

### 3. AGGREGATION DEVICES Section
```yaml
aggregation_devices:
  - hostname: SR201
    ip: 172.10.1.201
    device_type: ASR903
    role: aggregation
```

**What it is:** ASR903 PE routers at substations/sites  
**Function:** These terminate VRFs and connect to core  
**When needed:**
- ✅ **Pre-migration:** Capture their view of OLD core
- ✅ **During migration:** Monitor them during cutover (they stay connected!)
- ✅ **Post-migration:** Validate they see NEW core correctly

**Real-world example:**
```
Your network topology:

    Substations/Sites (Aggregation Layer)
    ┌──────────┐         ┌──────────┐
    │  SR201   │         │  SR202   │  <-- These are aggregation_devices
    │ ASR903   │         │ ASR903   │
    │ (Baliwag)│         │(Malolos) │
    └────┬─────┘         └────┬─────┘
         │                    │
         │  These links are  │
         │  being MIGRATED   │
         │                    │
    ┌────▼────────────────────▼─────┐
    │      CORE ROUTERS              │
    │  OLD ASR9010 → NEW ASR9906    │
    └────────────────────────────────┘
```

**Why you need this:**
- During migration, SR201/SR202 have links to BOTH old and new core
- Tests verify dual-homing works correctly
- Validates BGP sessions from aggregation perspective

**Important details:**
```yaml
connects_to:
  - UPE1              # New core
  - Legacy_ASR9010    # Old core (during migration)
```
This shows SR201 connects to BOTH during migration!

**Must change:** YES - `ip:` must match your SR201/SR202 IPs

---

### 4. LEGACY DEVICES Section
```yaml
legacy_devices:
  - hostname: OLD_ASR9K_DUHAT
    ip: 172.10.1.10
    device_type: ASR9010
    role: core_legacy
    status: to_be_decommissioned
```

**What it is:** Your OLD ASR9010 routers being replaced  
**Function:** Current production core with MPLS-LDP  
**When needed:**
- ✅ **Pre-migration:** Document OLD state for comparison
- ✅ **During migration:** Monitor Inter-AS link to new core
- ✅ **Rollback:** Target to revert to if migration fails

**Real-world example:**
```
BEFORE MIGRATION:
┌──────────────────────┐
│   LEGACY CORE        │
│  ┌────────────┐      │
│  │OLD_ASR9K   │      │  <-- This is what you're replacing
│  │  ASR9010   │      │      (legacy_devices)
│  └─────┬──────┘      │
└────────┼─────────────┘
         │
    All traffic goes through OLD core


DURING MIGRATION:
┌──────────────────────┐     ┌──────────────────────┐
│   LEGACY CORE        │     │    NEW CORE          │
│  ┌────────────┐      │     │  ┌────────────┐     │
│  │OLD_ASR9K   │◄─────┼─────┼─►│   UPE1     │     │
│  │  ASR9010   │ Inter│-AS  │  │  ASR9906   │     │
│  └─────┬──────┘ Link │     │  └─────┬──────┘     │
└────────┼─────────────┘     └────────┼─────────────┘
         │                            │
    Some traffic still here      Migrating to here
```

**Why you need this:**
- During migration, Inter-AS link between OLD and NEW is critical
- Tests validate traffic can flow through Inter-AS link
- Rollback target if something goes wrong

**inter_as_link explained:**
```yaml
inter_as_link:
  - interface: GigabitEthernet0/0/0/10
    connects_to: UPE1
    description: "Inter-AS link for migration"
```
This is the temporary link between old ASR9010 and new ASR9906!

**Must change:** YES - `ip:` must match your old ASR9010 IP

---

### 5. CRITICAL LINKS Section
```yaml
critical_links:
  - name: BLGTRPTPE1-DHTPASR9K
    source:
      device: SR201
      interface: GigabitEthernet0/0/0/0
    destination:
      device: UPE1
      interface: GigabitEthernet0/0/0/0
```

**What it is:** THE PHYSICAL LINKS YOU'RE MIGRATING  
**Function:** Documents exactly which fiber cables are being moved  
**When needed:**
- ✅ **Pre-migration:** Validate baseline state of these specific links
- ✅ **During migration:** THESE ARE THE ONES BEING PHYSICALLY MOVED!
- ✅ **Post-migration:** Validate they work on new core

**Real-world visualization:**
```
BEFORE CUTOVER:
SR201 Gi0/0/0/0 ─────fiber cable─────► OLD_ASR9K Gi0/0/0/5
                                       (legacy core)

DURING CUTOVER (Physical work):
1. Technician unplugs fiber from OLD_ASR9K Gi0/0/0/5
2. Technician plugs fiber into UPE1 Gi0/0/0/0
   
AFTER CUTOVER:
SR201 Gi0/0/0/0 ─────fiber cable─────► UPE1 Gi0/0/0/0
                                       (new core)
```

**Why critical:**
```yaml
migration_order: 1          # First link to migrate
cutover_time_minutes: 15    # Expected downtime
```

**Migration sequence:**
- **Link 1** (SR201→UPE1): Cutover at T+0, validate by T+15
- **Link 2** (SR202→UPE1): Cutover at T+30, validate by T+45

**Must change:** MAYBE - verify interface names match your hardware

---

### 6. ROLLBACK LINKS Section
```yaml
rollback_links:
  - name: ROLLBACK_LINK_SR201
    source:
      device: SR201
      interface: GigabitEthernet0/0/0/10
    destination:
      device: OLD_ASR9K_DUHAT
      interface: GigabitEthernet0/0/0/20
    status: shutdown    # CRITICAL: Must be shutdown!
```

**What it is:** Pre-installed backup links to OLD core  
**Function:** Emergency fallback if migration fails  
**When needed:**
- ✅ **Pre-migration:** Verify they're configured AND shutdown
- ⚠️ **During migration IF ROLLBACK NEEDED:** Activate these links
- ❌ **Post-migration (success):** Can be removed

**Real-world scenario:**
```
NORMAL SITUATION (Migration successful):
SR201 Gi0/0/0/0 ────► UPE1 Gi0/0/0/0  (Active)
SR201 Gi0/0/0/10 ───X OLD_ASR9K       (Shutdown - rollback link)

ROLLBACK SITUATION (Migration failed):
SR201 Gi0/0/0/0 ────X UPE1            (Disabled)
SR201 Gi0/0/0/10 ───► OLD_ASR9K       (Activated - emergency!)
```

**CRITICAL REQUIREMENTS:**
```yaml
status: shutdown            # MUST be shutdown before migration!
purpose: rollback_only      # Only activate if emergency
estimated_activation_time: 5  # 5 minutes to activate
```

**Why this exists:**
- If SCADA fails on new core → Quick rollback to old core
- If Telepro latency > 10ms → Quick rollback to old core
- Rollback link is PRE-CONFIGURED with same settings as active link

**Activation procedure:**
```yaml
activation_procedure:
  - "Remove fiber from new core"      # Unplug from UPE1
  - "Insert fiber to old core"        # Plug into OLD_ASR9K
  - "no shutdown on both ends"        # Enable interfaces
  - "Verify OSPF adjacency"           # Check routing
  - "Verify BGP sessions"             # Check services
```

**Must change:** YES - verify interface names are correct

---

### 7. ROUTING PROTOCOLS Section
```yaml
routing_protocols:
  ospf:
    process_id: 1
    hello_interval: 10
```

**What it is:** Standard routing protocol configuration  
**Function:** Reference for validation commands  
**When needed:** Tests use these values to verify correct configuration  

**Example usage in tests:**
```python
# Test checks if OSPF hello is 10 seconds
output = execute_command("show ospf interface")
verify(hello_interval == 10)
```

**Must change:** MAYBE - verify these match your actual config

---

### 8. MANAGEMENT ACCESS Section
```yaml
management:
  tacacs_servers:
    - ip: 10.240.100.1
  ntp_servers:
    - ip: 10.240.100.10
```

**What it is:** Centralized management services  
**Function:** Where devices send logs, sync time, authenticate users  
**When needed:**
- ✅ **Pre-migration:** Verify all devices can reach these services
- ✅ **During migration:** NOC monitors syslog for alerts
- ✅ **Post-migration:** Ongoing monitoring

**Real-world flow:**
```
All Routers → Send Syslog → 10.240.100.20 (NOC)
All Routers → Sync Time  → 10.240.100.10 (NTP)
All Routers → Auth Users → 10.240.100.1 (TACACS)
```

**Must change:** YES - use your actual NOC server IPs

---

### 9. MONITORING ENDPOINTS Section
```yaml
monitoring:
  critical_service_slas:
    scada:
      latency_max_ms: 200
      alert_priority: P1
```

**What it is:** SLA thresholds that trigger alerts  
**Function:** Defines acceptable performance for critical services  
**When needed:**
- ✅ **During migration:** Automatic alerts if thresholds exceeded
- ✅ **Post-migration:** Ongoing monitoring

**Real-world example:**
```
If Teleprotection latency > 10ms:
  → Send P1 alert to NOC
  → Page on-call engineer
  → Consider rollback

If SCADA connectivity < 100%:
  → Send P1 alert
  → Immediate investigation
```

**Must change:** MAYBE - verify thresholds match your SLAs

---

## 📊 Summary: What Each Section Does

| Section | Purpose | When Needed | Must Change? |
|---------|---------|-------------|--------------|
| **credentials** | SSH login | Every connection | ✅ YES |
| **core_devices** | NEW ASR9906 | All tests | ✅ YES (IPs) |
| **aggregation_devices** | ASR903 PE routers | All tests | ✅ YES (IPs) |
| **legacy_devices** | OLD ASR9010 | Pre/During/Rollback | ✅ YES (IPs) |
| **critical_links** | Links being migrated | During cutover | ⚠️ VERIFY |
| **rollback_links** | Emergency fallback | If rollback needed | ⚠️ VERIFY |
| **routing_protocols** | Config reference | Validation checks | ⚠️ VERIFY |
| **management** | NOC systems | Monitoring | ✅ YES (IPs) |
| **monitoring** | Alert thresholds | Real-time alerts | ⚠️ VERIFY |

---

## 🎯 Quick Start Checklist

1. **✅ Update Credentials**
   ```yaml
   credentials:
     production:
       username: YOUR_USERNAME
       password: YOUR_PASSWORD
   ```

2. **✅ Update Core Device IPs**
   ```yaml
   core_devices:
     - hostname: UPE1
       ip: YOUR_UPE1_IP    # <-- Change this!
   ```

3. **✅ Update Aggregation IPs**
   ```yaml
   aggregation_devices:
     - hostname: SR201
       ip: YOUR_SR201_IP   # <-- Change this!
   ```

4. **✅ Update Legacy IP**
   ```yaml
   legacy_devices:
     - hostname: OLD_ASR9K_DUHAT
       ip: YOUR_OLD_ASR9K_IP  # <-- Change this!
   ```

5. **⚠️ Verify Interface Names**
   ```yaml
   critical_links:
     source:
       interface: GigabitEthernet0/0/0/0  # <-- Verify correct!
   ```

6. **✅ Update NOC IPs**
   ```yaml
   management:
     syslog_servers:
       - ip: YOUR_NOC_IP    # <-- Change this!
   ```

---

## 💡 Pro Tips

### Test Connectivity First
```bash
# Before running full tests, verify you can connect:
ssh YOUR_USERNAME@172.10.1.1

# If this works, your credentials are correct!
```

### Minimal Required Changes
To get tests running in your lab, you only need to change:
1. ✅ Credentials (username/password)
2. ✅ Device IPs (core, aggregation, legacy)

Everything else can stay as-is initially!

### Incremental Approach
1. Update credentials and IPs only
2. Run pre-migration baseline
3. Fix any failures
4. Then refine other sections as needed

---

**Now you know exactly what each section does and when it's needed!** 🎉
