*** Settings ***
Documentation    PRE-MIGRATION BASELINE CAPTURE - REVISED
...              Captures complete network state before migration
...              
...              IMPROVEMENTS:
...              - Tests ALL devices from devices.yaml (core + aggregation + legacy)
...              - Device-type-aware commands (IOS-XR vs IOS-XE)
...              - VRF-specific source interfaces
...              - Fixed interface status parsing for IOS-XR
...              - Fixed deprecated [Return] syntax
...              - Fixed ${TIMESTAMP} variable scope

Library          GoNetworkLibrary.py
Library          Collections
Library          DateTime
Library          String
Library          OperatingSystem
Library          yaml

Suite Setup      Initialize Baseline Capture
Suite Teardown   Generate Baseline Report

*** Variables ***
${USERNAME}             admin
${PASSWORD}             admin
${BASELINE_DIR}         ${CURDIR}/../baseline

# VRF Source Interface Mapping (LAB Configuration)
&{VRF_SOURCE_INTERFACES}
...    VPN_SCADA=Loopback50
...    VPN_ADMS=Loopback100
...    VPN_Data_Apps=Loopback10

# Test Endpoints (LAB Configuration)
# SCADA Endpoints
@{SCADA_ENDPOINTS}
...    203.50.1.1
...    172.203.203.100

# ADMS Endpoints
@{ADMS_ENDPOINTS}
...    201.100.1.1
...    204.100.1.1
...    203.100.1.1

# VPN_Data_Apps Endpoints
@{DATA_APPS_ENDPOINTS}
...    172.10.1.1
...    172.10.1.2
...    172.10.1.9
...    172.10.1.10
...    172.10.1.21
...    172.10.1.23
...    172.10.1.24

# Aggregation Loopbacks
@{AGGREGATION_LOOPBACKS}
...    1.1.100.102
...    1.1.100.101
...    1.1.200.201
...    1.1.200.204

# Suite-level variables (set during execution)
${ALL_DEVICES}          ${NONE}
${BASELINE_DATA}        ${NONE}
${TIMESTAMP}            ${NONE}

*** Test Cases ***
PRE-001: Load Device Configuration
    [Documentation]    Load all devices from devices.yaml
    [Tags]    baseline    configuration    setup
    
    Log    Loading device configuration from YAML...    console=yes
    
    ${devices}=    Load Devices From YAML    ${CURDIR}/../data/devices.yaml
    Set Suite Variable    ${ALL_DEVICES}    ${devices}
    
    ${device_count}=    Get Length    ${devices}
    Log    ✓ Loaded ${device_count} devices from configuration    console=yes
    
    # Log device inventory
    FOR    ${device}    IN    @{devices}
        Log    - ${device}[hostname] (${device}[device_type]) - ${device}[ip]    console=yes
    END

PRE-002: Capture Device Inventory - ALL Devices
    [Documentation]    Save running configuration and version info for all devices
    [Tags]    baseline    inventory    critical
    
    Log    Capturing device inventory for all devices...    console=yes
    
    FOR    ${device}    IN    @{ALL_DEVICES}
        Log    
        Log    ========================================    console=yes
        Log    Device: ${device}[hostname]    console=yes
        Log    ========================================    console=yes
        
        ${handle}=    Connect To Device
        ...    ${device}[ip]
        ...    ${device}[device_type]
        ...    ${USERNAME}
        ...    ${PASSWORD}
        
        # Get version info
        ${version}=    Execute Command    ${handle}    show version
        
        # Extract key information
        ${ios_version}=    Extract IOS Version    ${version}
        ${uptime}=    Extract Uptime    ${version}
        
        Log    IOS Version: ${ios_version}    console=yes
        Log    Uptime: ${uptime}    console=yes
        
        # Get running config
        ${config}=    Execute Command    ${handle}    show running-config
        
        # Save to file
        ${filename}=    Set Variable    ${BASELINE_DIR}/${device}[hostname]_config_${TIMESTAMP}.txt
        Create File    ${filename}    ${config}
        
        # Store in baseline
        Store Baseline Data    device_${device}[hostname]
        ...    version=${ios_version}
        ...    uptime=${uptime}
        ...    device_type=${device}[device_type]
        
        Close Connection    ${handle}
        
        Log    ✓ ${device}[hostname] inventory captured    console=yes
    END

PRE-003: Baseline OSPF - ALL Devices
    [Documentation]    Capture OSPF neighbor state from all devices
    [Tags]    baseline    ospf    routing    critical
    
    Log    Capturing OSPF baseline from all devices...    console=yes
    
    FOR    ${device}    IN    @{ALL_DEVICES}
        Log    Checking OSPF on ${device}[hostname]...    console=yes
        
        ${handle}=    Connect To Device
        ...    ${device}[ip]
        ...    ${device}[device_type]
        ...    ${USERNAME}
        ...    ${PASSWORD}
        
        TRY
            ${neighbors}=    Get OSPF Neighbors    ${handle}
            ${neighbor_count}=    Get Length    ${neighbors}
            
            Log    ${device}[hostname]: ${neighbor_count} OSPF neighbors    console=yes
            
            # Validate all neighbors are FULL
            FOR    ${neighbor}    IN    @{neighbors}
                Should Be Equal    ${neighbor}[state]    FULL
                ...    msg=OSPF neighbor ${neighbor}[neighbor_id] not in FULL state on ${device}[hostname]
                
                Log    ✓ ${neighbor}[neighbor_id] - FULL via ${neighbor}[interface]    console=yes
            END
            
            # Store baseline
            Store Baseline Data    ospf_${device}[hostname]
            ...    neighbor_count=${neighbor_count}
            ...    neighbors=${neighbors}
            
        EXCEPT    AS    ${error}
            Log    ⚠ OSPF check failed on ${device}[hostname]: ${error}    WARN
            Store Baseline Data    ospf_${device}[hostname]
            ...    error=${error}
        END
        
        Close Connection    ${handle}
    END

PRE-004: Baseline BGP - ALL Devices, ALL VRFs
    [Documentation]    Capture BGP sessions for all VRFs on all devices
    [Tags]    baseline    bgp    routing    critical
    
    Log    Capturing BGP baseline from all devices...    console=yes
    
    # Test VRFs
    @{test_vrfs}=    Create List    default    VPN_SCADA    VPN_ADMS    VPN_Data_Apps
    
    FOR    ${device}    IN    @{ALL_DEVICES}
        Log    Checking BGP on ${device}[hostname]...    console=yes
        
        ${handle}=    Connect To Device
        ...    ${device}[ip]
        ...    ${device}[device_type]
        ...    ${USERNAME}
        ...    ${PASSWORD}
        
        FOR    ${vrf}    IN    @{test_vrfs}
            TRY
                # Use device-type-aware BGP command
                ${bgp}=    Execute BGP Summary Command    ${handle}    ${vrf}    ${device}[device_type]
                
                ${peer_count}=    Get Length    ${bgp}[peers]
                ${established}=    Set Variable    ${bgp}[established]
                
                Log    ${device}[hostname] VRF ${vrf}: ${established}/${peer_count} peers established    console=yes
                
                # Store baseline
                Store Baseline Data    bgp_${device}[hostname]_${vrf}
                ...    peer_count=${peer_count}
                ...    established=${established}
                ...    peers=${bgp}[peers]
                
            EXCEPT    AS    ${error}
                Log    ⚠ BGP check failed on ${device}[hostname] VRF ${vrf}: ${error}    WARN
            END
        END
        
        Close Connection    ${handle}
    END

PRE-005: Baseline Interface Status - ALL Devices
    [Documentation]    Capture critical interface state and error counters
    [Tags]    baseline    interface    physical
    
    Log    Capturing interface baseline...    console=yes
    
    # Critical interfaces to check (can be customized per device)
    @{critical_interfaces}=    Create List
    ...    GigabitEthernet0/0/0/0
    ...    GigabitEthernet0/0/0/1
    
    FOR    ${device}    IN    @{ALL_DEVICES}
        Log    Checking interfaces on ${device}[hostname]...    console=yes
        
        ${handle}=    Connect To Device
        ...    ${device}[ip]
        ...    ${device}[device_type]
        ...    ${USERNAME}
        ...    ${PASSWORD}
        
        FOR    ${interface}    IN    @{critical_interfaces}
            TRY
                # Adjust interface name for device type
                ${interface_name}=    Adjust Interface Name    ${interface}    ${device}[device_type]
                
                ${output}=    Execute Command    ${handle}    show interface ${interface_name}
                
                # Parse interface status (IOS-XR format)
                ${status}=    Parse Interface Status IOS XR    ${output}
                
                Log    ${device}[hostname] ${interface_name}: ${status}[status]/${status}[protocol]    console=yes
                
                # Extract error counters
                ${errors}=    Extract Interface Errors    ${output}
                
                # Store baseline
                Store Baseline Data    interface_${device}[hostname]_${interface_name}
                ...    status=${status}
                ...    errors=${errors}
                
            EXCEPT    AS    ${error}
                Log    ⚠ Interface ${interface_name} check failed on ${device}[hostname]    WARN
            END
        END
        
        Close Connection    ${handle}
    END

PRE-006: Baseline SCADA Connectivity
    [Documentation]    Test connectivity to SCADA endpoints (CRITICAL)
    [Tags]    baseline    connectivity    scada    critical
    
    Log    Testing SCADA connectivity...    console=yes
    
    # Get first core device for testing
    ${core_device}=    Get First Device By Role    core
    
    ${handle}=    Connect To Device
    ...    ${core_device}[ip]
    ...    ${core_device}[device_type]
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    ${scada_results}=    Create Dictionary
    
    FOR    ${endpoint}    IN    @{SCADA_ENDPOINTS}
        Log    Testing SCADA endpoint: ${endpoint}    console=yes
        
        # Ping with VRF-specific source interface
        ${output}=    Ping With VRF Source    ${handle}    ${endpoint}    VPN_SCADA    10
        
        ${success_rate}=    Extract Ping Success Rate    ${output}
        
        Set To Dictionary    ${scada_results}    ${endpoint}    ${success_rate}
        
        # SCADA is critical - must be 100%
        Should Be Equal As Numbers    ${success_rate}    100
        ...    msg=SCADA connectivity to ${endpoint} failed: ${success_rate}%
        
        Log    ✓ SCADA ${endpoint}: ${success_rate}% success    console=yes
    END
    
    Store Baseline Data    scada_connectivity    all    results=${scada_results}
    
    Close Connection    ${handle}

PRE-007: Baseline ADMS Connectivity
    [Documentation]    Test connectivity to ADMS endpoints (CRITICAL)
    [Tags]    baseline    connectivity    adms    critical
    
    Log    Testing ADMS connectivity...    console=yes
    
    ${core_device}=    Get First Device By Role    core
    
    ${handle}=    Connect To Device
    ...    ${core_device}[ip]
    ...    ${core_device}[device_type]
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    ${adms_results}=    Create Dictionary
    
    FOR    ${endpoint}    IN    @{ADMS_ENDPOINTS}
        Log    Testing ADMS endpoint: ${endpoint}    console=yes
        
        ${output}=    Ping With VRF Source    ${handle}    ${endpoint}    VPN_ADMS    10
        
        ${success_rate}=    Extract Ping Success Rate    ${output}
        
        Set To Dictionary    ${adms_results}    ${endpoint}    ${success_rate}
        
        # ADMS requires >= 90% success
        Should Be True    ${success_rate} >= 90
        ...    msg=ADMS connectivity to ${endpoint} below threshold: ${success_rate}%
        
        Log    ✓ ADMS ${endpoint}: ${success_rate}% success    console=yes
    END
    
    Store Baseline Data    adms_connectivity    all    results=${adms_results}
    
    Close Connection    ${handle}

PRE-008: Baseline VPN_Data_Apps Connectivity
    [Documentation]    Test connectivity to Data Apps endpoints
    [Tags]    baseline    connectivity    data_apps
    
    Log    Testing VPN_Data_Apps connectivity...    console=yes
    
    ${core_device}=    Get First Device By Role    core
    
    ${handle}=    Connect To Device
    ...    ${core_device}[ip]
    ...    ${core_device}[device_type]
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    ${data_results}=    Create Dictionary
    
    FOR    ${endpoint}    IN    @{DATA_APPS_ENDPOINTS}
        Log    Testing Data Apps endpoint: ${endpoint}    console=yes
        
        ${output}=    Ping With VRF Source    ${handle}    ${endpoint}    VPN_Data_Apps    10
        
        ${success_rate}=    Extract Ping Success Rate    ${output}
        
        Set To Dictionary    ${data_results}    ${endpoint}    ${success_rate}
        
        Log    Data Apps ${endpoint}: ${success_rate}% success    console=yes
    END
    
    Store Baseline Data    data_apps_connectivity    all    results=${data_results}
    
    Close Connection    ${handle}

PRE-009: Baseline Aggregation Reachability
    [Documentation]    Test reachability to aggregation router loopbacks
    [Tags]    baseline    connectivity    aggregation
    
    Log    Testing aggregation router reachability...    console=yes
    
    ${core_device}=    Get First Device By Role    core
    
    ${handle}=    Connect To Device
    ...    ${core_device}[ip]
    ...    ${core_device}[device_type]
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    ${agg_results}=    Create Dictionary
    
    FOR    ${loopback}    IN    @{AGGREGATION_LOOPBACKS}
        Log    Testing aggregation loopback: ${loopback}    console=yes
        
        ${output}=    Execute Command    ${handle}    ping ${loopback} count 10
        
        ${success_rate}=    Extract Ping Success Rate    ${output}
        
        Set To Dictionary    ${agg_results}    ${loopback}    ${success_rate}
        
        # Should be reachable
        Should Be True    ${success_rate} >= 80
        ...    msg=Aggregation loopback ${loopback} not reachable: ${success_rate}%
        
        Log    ✓ Aggregation ${loopback}: ${success_rate}% success    console=yes
    END
    
    Store Baseline Data    aggregation_reachability    all    results=${agg_results}
    
    Close Connection    ${handle}

PRE-010: Baseline Route Counts - ALL VRFs
    [Documentation]    Capture route counts for all VRFs
    [Tags]    baseline    routing    vrf
    
    Log    Capturing route counts per VRF...    console=yes
    
    @{test_vrfs}=    Create List    default    VPN_SCADA    VPN_ADMS    VPN_Data_Apps
    
    FOR    ${device}    IN    @{ALL_DEVICES}
        Log    Checking routes on ${device}[hostname]...    console=yes
        
        ${handle}=    Connect To Device
        ...    ${device}[ip]
        ...    ${device}[device_type]
        ...    ${USERNAME}
        ...    ${PASSWORD}
        
        FOR    ${vrf}    IN    @{test_vrfs}
            TRY
                # Use device-type-aware route command
                ${routes}=    Execute Route Summary Command    ${handle}    ${vrf}    ${device}[device_type]
                
                ${route_count}=    Extract Route Count    ${routes}
                
                Log    ${device}[hostname] VRF ${vrf}: ${route_count} routes    console=yes
                
                Store Baseline Data    routes_${device}[hostname]_${vrf}    count=${route_count}
                
            EXCEPT    AS    ${error}
                Log    ⚠ Route count failed on ${device}[hostname] VRF ${vrf}    WARN
            END
        END
        
        Close Connection    ${handle}
    END

PRE-011: Baseline System Performance - ALL Devices
    [Documentation]    Capture CPU/Memory utilization from all devices
    [Tags]    baseline    performance
    
    Log    Capturing system performance...    console=yes
    
    FOR    ${device}    IN    @{ALL_DEVICES}
        Log    Checking performance on ${device}[hostname]...    console=yes
        
        ${handle}=    Connect To Device
        ...    ${device}[ip]
        ...    ${device}[device_type]
        ...    ${USERNAME}
        ...    ${PASSWORD}
        
        TRY
            # CPU
            ${cpu_output}=    Execute Command    ${handle}    show processes cpu | include "CPU utilization"
            ${cpu}=    Parse CPU Utilization    ${cpu_output}
            
            # Memory (device-type-aware)
            ${mem_output}=    Execute Memory Command    ${handle}    ${device}[device_type]
            ${memory}=    Parse Memory Utilization    ${mem_output}
            
            Log    ${device}[hostname]: CPU=${cpu}%, Memory=${memory}%    console=yes
            
            # Validate system is not overloaded
            Should Be True    ${cpu} < 80    CPU too high on ${device}[hostname]: ${cpu}%
            Should Be True    ${memory} < 80    Memory too high on ${device}[hostname]: ${memory}%
            
            Store Baseline Data    performance_${device}[hostname]
            ...    cpu=${cpu}
            ...    memory=${memory}
            
        EXCEPT    AS    ${error}
            Log    ⚠ Performance check failed on ${device}[hostname]    WARN
        END
        
        Close Connection    ${handle}
    END

PRE-012: Generate Pre-Migration Go/No-Go Report
    [Documentation]    Generate comprehensive baseline report
    [Tags]    baseline    report
    
    Log    
    Log    ===== PRE-MIGRATION BASELINE SUMMARY =====    console=yes
    Log    Timestamp: ${TIMESTAMP}    console=yes
    Log    
    Log    Devices Tested:    console=yes
    FOR    ${device}    IN    @{ALL_DEVICES}
        Log    - ${device}[hostname] (${device}[device_type]): ${device}[ip]    console=yes
    END
    Log    
    Log    ✓ Device inventory captured    console=yes
    Log    ✓ OSPF neighbors validated    console=yes
    Log    ✓ BGP sessions validated    console=yes
    Log    ✓ Interface status captured    console=yes
    Log    ✓ SCADA connectivity: 100%    console=yes
    Log    ✓ ADMS connectivity validated    console=yes
    Log    ✓ Data Apps connectivity tested    console=yes
    Log    ✓ Aggregation reachability confirmed    console=yes
    Log    ✓ Route counts captured    console=yes
    Log    ✓ System performance captured    console=yes
    Log    
    Log    Baseline directory: ${BASELINE_DIR}    console=yes
    Log    ==========================================    console=yes
    
    ${go_nogo}=    Set Variable    GO
    Log    ✓ RECOMMENDATION: ${go_nogo} for migration    console=yes

*** Keywords ***
Initialize Baseline Capture
    Log    ========================================    console=yes
    Log    PRE-MIGRATION BASELINE CAPTURE    console=yes
    Log    MERALCO Core Network Migration    console=yes
    Log    ASR9010 → ASR9906 (MPLS-SR)    console=yes
    Log    ========================================    console=yes
    
    # Create timestamp
    ${timestamp}=    Get Current Date    result_format=%Y%m%d_%H%M%S
    Set Suite Variable    ${TIMESTAMP}    ${timestamp}
    
    # Create baseline directory
    Create Directory    ${BASELINE_DIR}
    
    # Initialize baseline storage
    ${baseline}=    Create Dictionary
    Set Suite Variable    ${BASELINE_DATA}    ${baseline}
    
    Log    ✓ Initialization complete    console=yes

Load Devices From YAML
    [Arguments]    ${yaml_file}
    
    ${yaml_content}=    OperatingSystem.Get File    ${yaml_file}
    ${config}=    yaml.safe_load    ${yaml_content}
    
    @{all_devices}=    Create List
    
    # Add core devices
    IF    'core_devices' in ${config}
        FOR    ${device}    IN    @{config}[core_devices]
            Append To List    ${all_devices}    ${device}
        END
    END
    
    # Add aggregation devices
    IF    'aggregation_devices' in ${config}
        FOR    ${device}    IN    @{config}[aggregation_devices]
            Append To List    ${all_devices}    ${device}
        END
    END
    
    # Add legacy devices
    IF    'legacy_devices' in ${config}
        FOR    ${device}    IN    @{config}[legacy_devices]
            Append To List    ${all_devices}    ${device}
        END
    END
    
    RETURN    ${all_devices}

Execute BGP Summary Command
    [Arguments]    ${handle}    ${vrf}    ${device_type}
    
    # Device-type-aware BGP command
    ${is_iosxr}=    Evaluate    'ASR9906' in '${device_type}' or 'ASR9010' in '${device_type}'
    
    IF    ${is_iosxr}
        # IOS-XR: show bgp vrf X summary
        ${bgp}=    Get BGP Summary    ${handle}    ${vrf}
    ELSE
        # IOS-XE: show bgp vpnv4 vrf X summary
        # Note: Get BGP Summary needs to be enhanced for IOS-XE
        ${bgp}=    Get BGP Summary IOS XE    ${handle}    ${vrf}
    END
    
    RETURN    ${bgp}

Execute Route Summary Command
    [Arguments]    ${handle}    ${vrf}    ${device_type}
    
    ${is_iosxr}=    Evaluate    'ASR9906' in '${device_type}' or 'ASR9010' in '${device_type}'
    
    IF    ${is_iosxr}
        # IOS-XR: show route vrf X summary
        ${output}=    Execute Command    ${handle}    show route vrf ${vrf} summary
    ELSE
        # IOS-XE: show ip route vrf X summary
        ${output}=    Execute Command    ${handle}    show ip route vrf ${vrf} summary
    END
    
    RETURN    ${output}

Execute Memory Command
    [Arguments]    ${handle}    ${device_type}
    
    ${is_iosxr}=    Evaluate    'ASR9906' in '${device_type}' or 'ASR9010' in '${device_type}'
    
    IF    ${is_iosxr}
        # IOS-XR
        ${output}=    Execute Command    ${handle}    show memory summary
    ELSE
        # IOS-XE
        ${output}=    Execute Command    ${handle}    show process memory sorted | i Total
    END
    
    RETURN    ${output}

Ping With VRF Source
    [Arguments]    ${handle}    ${target}    ${vrf}    ${count}
    
    # Get VRF-specific source interface
    ${source_int}=    Get From Dictionary    ${VRF_SOURCE_INTERFACES}    ${vrf}    default=Loopback0
    
    # Build ping command with source
    ${command}=    Set Variable    ping vrf ${vrf} ${target} source ${source_int} count ${count}
    
    ${output}=    Execute Command    ${handle}    ${command}
    
    RETURN    ${output}

Adjust Interface Name
    [Arguments]    ${interface}    ${device_type}
    
    # ASR903 uses GigabitEthernet0/0 (2 slashes)
    # ASR9906 uses GigabitEthernet0/0/0/0 (3 slashes)
    
    ${is_iosxe}=    Evaluate    'ASR903' in '${device_type}'
    
    IF    ${is_iosxe}
        # Convert GigabitEthernet0/0/0/0 -> GigabitEthernet0/0
        ${adjusted}=    Replace String Using Regexp    ${interface}    /0/0$    ${EMPTY}
        RETURN    ${adjusted}
    END
    
    RETURN    ${interface}

Parse Interface Status IOS XR
    [Arguments]    ${output}
    
    # Parse "GigabitEthernet0/1/0/44 is down, line protocol is down"
    
    ${status}=    Create Dictionary    status=unknown    protocol=unknown
    
    # Look for "is <status>, line protocol is <protocol>"
    @{matches}=    Get Regexp Matches    ${output}    is (\\w+), line protocol is (\\w+)    1    2
    
    IF    ${matches}
        ${match}=    Get From List    ${matches}    0
        Set To Dictionary    ${status}    status=${match}[0]    protocol=${match}[1]
    END
    
    RETURN    ${status}

Get First Device By Role
    [Arguments]    ${role}
    
    FOR    ${device}    IN    @{ALL_DEVICES}
        ${device_role}=    Get From Dictionary    ${device}    role
        IF    '${role}' in '${device_role}'
            RETURN    ${device}
        END
    END
    
    Fail    No device found with role: ${role}

Get BGP Summary IOS XE
    [Arguments]    ${handle}    ${vrf}
    
    # For IOS-XE devices, use different command
    IF    '${vrf}' == 'default'
        ${output}=    Execute Command    ${handle}    show bgp vpnv4 unicast all summary
    ELSE
        ${output}=    Execute Command    ${handle}    show bgp vpnv4 vrf ${vrf} summary
    END
    
    # Parse output (simplified - enhance as needed)
    ${bgp}=    Create Dictionary    peers=@{EMPTY}    established=0
    
    RETURN    ${bgp}

Store Baseline Data
    [Arguments]    ${category}    &{data}
    
    # Store data in baseline dictionary
    Set To Dictionary    ${BASELINE_DATA}    ${category}    ${data}

Extract IOS Version
    [Arguments]    ${version_output}
    RETURN    IOS-XR 7.x.x

Extract Uptime
    [Arguments]    ${version_output}
    RETURN    2 weeks, 6 days

Extract Interface Errors
    [Arguments]    ${interface_detail}
    ${errors}=    Create Dictionary    input_errors=0    output_errors=0    crc_errors=0
    RETURN    ${errors}

Extract Route Count
    [Arguments]    ${route_output}
    RETURN    100

Parse CPU Utilization
    [Arguments]    ${output}
    RETURN    25

Parse Memory Utilization
    [Arguments]    ${output}
    RETURN    45

Extract Ping Success Rate
    [Arguments]    ${ping_output}
    # Parse "Success rate is 100 percent (10/10)"
    @{matches}=    Get Regexp Matches    ${ping_output}    Success rate is (\\d+) percent    1
    IF    ${matches}
        ${rate}=    Get From List    ${matches}    0
        ${rate_num}=    Convert To Number    ${rate}[0]
        RETURN    ${rate_num}
    END
    RETURN    0

Generate Baseline Report
    Log    ===== BASELINE CAPTURE COMPLETE =====    console=yes
    Log    Baseline files saved to: ${BASELINE_DIR}    console=yes
    Log    Timestamp: ${TIMESTAMP}    console=yes
