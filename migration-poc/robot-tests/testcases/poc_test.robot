*** Settings ***
Documentation    POC Test Suite - Network Migration Validation
...              This suite demonstrates Go Remote Library integration with Robot Framework
...              
...              Device Inventory:
...              - Core Routers: UPE1, UPE9, UPE21, UPE24 (ASR9906)
...              - Aggregation: SR201, SR202, SR203, SR204 (ASR903)
...              - Switch: Switch1 (Cat9300)

Library          Remote    http://localhost:8270    WITH NAME    GoLib
Library          Collections
Library          OperatingSystem
Variables        ../data/devices.yaml

Suite Setup      Log Test Environment
Suite Teardown   Log    ===== Test Suite Completed =====    console=yes

*** Variables ***
${USERNAME}      admin
${PASSWORD}      admin

*** Test Cases ***
TEST-001: Verify Go Remote Library Connection
    [Documentation]    Verify that Robot Framework can communicate with Go library
    [Tags]    smoke    connectivity
    
    Log    Testing connection to Go Remote Library on port 8270    console=yes
    ${keywords}=    GoLib.Get Keyword Names
    Log    Available keywords: ${keywords}    console=yes
    Should Not Be Empty    ${keywords}
    Log    ✓ Go Remote Library is responsive    console=yes

TEST-002: Connect to Core Router UPE1
    [Documentation]    Test SSH connection to ASR9906 core router
    [Tags]    connection    core
    
    Log    Connecting to UPE1 (172.10.1.1)...    console=yes
    
    ${handle}=    GoLib.Connect To Device
    ...    172.10.1.1
    ...    ASR9906
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    Should Not Be Empty    ${handle}
    Log    ✓ Connected successfully, handle: ${handle}    console=yes
    Set Suite Variable    ${UPE1_HANDLE}    ${handle}

TEST-003: Execute Show Version on UPE1
    [Documentation]    Verify command execution on IOS-XR device
    [Tags]    command    core
    
    Log    Executing 'show version' on UPE1...    console=yes
    
    ${output}=    GoLib.Execute Command
    ...    ${UPE1_HANDLE}
    ...    show version
    
    Should Contain    ${output}    Cisco IOS XR Software
    Log    ✓ Command executed successfully    console=yes
    Log    Output preview: ${output[:200]}...    console=yes

TEST-004: Get OSPF Neighbors on UPE1
    [Documentation]    Retrieve and validate OSPF neighbor information
    [Tags]    ospf    routing    core
    
    Log    Retrieving OSPF neighbors from UPE1...    console=yes
    
    ${neighbors}=    GoLib.Get OSPF Neighbors    ${UPE1_HANDLE}
    
    Log    Found ${neighbors.__len__()}  OSPF neighbors    console=yes
    
    # Validate we have neighbors
    Run Keyword If    ${neighbors.__len__()} > 0
    ...    Log    ✓ OSPF neighbors detected    console=yes
    ...    ELSE
    ...    Log    ⚠ No OSPF neighbors found (may be expected in isolated lab)    WARN
    
    # Log neighbor details
    FOR    ${neighbor}    IN    @{neighbors}
        Log    Neighbor: ${neighbor}[neighbor_id] - State: ${neighbor}[state] - Interface: ${neighbor}[interface]    console=yes
    END

TEST-005: Check BGP Summary on UPE1
    [Documentation]    Retrieve BGP peer status
    [Tags]    bgp    routing    core
    
    Log    Retrieving BGP summary from UPE1...    console=yes
    
    ${bgp}=    GoLib.Get BGP Summary    ${UPE1_HANDLE}    default
    
    Log    BGP Peers: ${bgp}[peers].__len__()    console=yes
    Log    Established: ${bgp}[established]    console=yes
    
    # Log peer details
    Run Keyword If    ${bgp}[peers].__len__() > 0
    ...    Log    ✓ BGP peers configured    console=yes
    ...    ELSE
    ...    Log    ⚠ No BGP peers found (may be expected in isolated lab)    WARN

TEST-006: Connect to Aggregation Router SR201
    [Documentation]    Test SSH connection to ASR903 aggregation router
    [Tags]    connection    aggregation
    
    Log    Connecting to SR201 (172.10.1.201)...    console=yes
    
    ${handle}=    GoLib.Connect To Device
    ...    172.10.1.201
    ...    ASR903
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    Should Not Be Empty    ${handle}
    Log    ✓ Connected successfully, handle: ${handle}    console=yes
    Set Suite Variable    ${SR201_HANDLE}    ${handle}

TEST-007: Execute Command on ASR903
    [Documentation]    Verify command execution on IOS-XE device
    [Tags]    command    aggregation
    
    Log    Executing 'show version' on SR201...    console=yes
    
    ${output}=    GoLib.Execute Command
    ...    ${SR201_HANDLE}
    ...    show version
    
    # IOS-XE devices show "Cisco IOS Software" or "Cisco IOS-XE Software"
    Should Contain Any    ${output}    Cisco IOS    IOS-XE
    Log    ✓ Command executed successfully    console=yes

TEST-008: Ping Test Between Devices
    [Documentation]    Test connectivity using ping through Go library
    [Tags]    connectivity    ping
    
    Log    Testing ping from UPE1 to SR201...    console=yes
    
    ${result}=    GoLib.Ping Test
    ...    ${UPE1_HANDLE}
    ...    172.10.1.201
    ...    default
    ...    5
    
    Log    Ping results: ${result}    console=yes
    Log    Success rate: ${result}[success_pct]%    console=yes
    
    # Validate connectivity
    Should Be True    ${result}[success_pct] >= 80
    Log    ✓ Ping test successful    console=yes

TEST-009: Check Interface Status
    [Documentation]    Verify interface operational status
    [Tags]    interface    status
    
    Log    Checking interface status on UPE1...    console=yes
    
    # Check a common interface (adjust based on your lab)
    ${status}=    GoLib.Get Interface Status
    ...    ${UPE1_HANDLE}
    ...    GigabitEthernet0/0/0/0
    
    Log    Interface status: ${status}    console=yes
    
    Run Keyword If    '${status}[status]' == 'up'
    ...    Log    ✓ Interface is UP    console=yes
    ...    ELSE
    ...    Log    ⚠ Interface is ${status}[status]    WARN

TEST-010: Multi-Device Health Check
    [Documentation]    Perform health check across multiple devices
    [Tags]    health-check    parallel
    
    Log    Performing health check on all core routers...    console=yes
    
    @{core_devices}=    Create List    
    ...    172.10.1.1    # UPE1
    ...    172.10.1.9    # UPE9
    
    FOR    ${device_ip}    IN    @{core_devices}
        Log    Checking ${device_ip}...    console=yes
        
        ${handle}=    GoLib.Connect To Device
        ...    ${device_ip}
        ...    ASR9906
        ...    ${USERNAME}
        ...    ${PASSWORD}
        
        ${version}=    GoLib.Execute Command    ${handle}    show version
        Should Contain    ${version}    IOS XR
        
        ${ospf}=    GoLib.Get OSPF Neighbors    ${handle}
        Log    ${device_ip}: ${ospf.__len__()} OSPF neighbors    console=yes
        
        GoLib.Close Connection    ${handle}
    END
    
    Log    ✓ Multi-device health check completed    console=yes

*** Keywords ***
Log Test Environment
    Log    ===== POC TEST SUITE STARTING =====    console=yes
    Log    Go Remote Library: localhost:8270    console=yes
    Log    Username: ${USERNAME}    console=yes
    Log    ==========================================    console=yes
    Log    Available Test Devices:    console=yes
    Log    Core: UPE1, UPE9, UPE21, UPE24    console=yes
    Log    Aggregation: SR201, SR202, SR203, SR204    console=yes
    Log    Switch: Switch1    console=yes
    Log    ==========================================    console=yes
