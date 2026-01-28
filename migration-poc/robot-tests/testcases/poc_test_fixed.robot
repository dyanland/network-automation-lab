*** Settings ***
Documentation    POC Test Suite - Using Custom GoNetworkLibrary
...              This version uses direct JSON-RPC instead of Robot Framework Remote library

Library          GoNetworkLibrary.py
Library          Collections
Variables        ../data/devices.yaml

Suite Setup      Log Test Environment
Suite Teardown   Log    ===== Test Suite Completed =====    console=yes

*** Variables ***
${USERNAME}      admin
${PASSWORD}      admin

*** Test Cases ***
TEST-001: Connect to Core Router UPE1
    [Documentation]    Test SSH connection to ASR9906 core router
    [Tags]    connection    core
    
    Log    Connecting to UPE1 (172.10.1.1)...    console=yes
    
    ${handle}=    Connect To Device
    ...    172.10.1.1
    ...    ASR9906
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    Should Not Be Empty    ${handle}
    Log    ✓ Connected successfully, handle: ${handle}    console=yes
    Set Suite Variable    ${UPE1_HANDLE}    ${handle}

TEST-002: Execute Show Version on UPE1
    [Documentation]    Verify command execution on IOS-XR device
    [Tags]    command    core
    
    Log    Executing 'show version' on UPE1...    console=yes
    
    ${output}=    Execute Command
    ...    ${UPE1_HANDLE}
    ...    show version
    
    Should Contain    ${output}    Cisco IOS XR Software
    Log    ✓ Command executed successfully    console=yes
    Log    Output preview: ${output[:200]}...    console=yes

TEST-003: Get OSPF Neighbors on UPE1
    [Documentation]    Retrieve and validate OSPF neighbor information
    [Tags]    ospf    routing    core
    
    Log    Retrieving OSPF neighbors from UPE1...    console=yes
    
    ${neighbors}=    Get OSPF Neighbors    ${UPE1_HANDLE}
    
    ${count}=    Get Length    ${neighbors}
    Log    Found ${count} OSPF neighbors    console=yes
    
    # Validate we have neighbors
    Run Keyword If    ${count} > 0
    ...    Log    ✓ OSPF neighbors detected    console=yes
    ...    ELSE
    ...    Log    ⚠ No OSPF neighbors found (may be expected in isolated lab)    WARN
    
    # Log neighbor details
    FOR    ${neighbor}    IN    @{neighbors}
        Log    Neighbor: ${neighbor}[neighbor_id] - State: ${neighbor}[state] - Interface: ${neighbor}[interface]    console=yes
    END

TEST-004: Check BGP Summary on UPE1
    [Documentation]    Retrieve BGP peer status
    [Tags]    bgp    routing    core
    
    Log    Retrieving BGP summary from UPE1...    console=yes
    
    ${bgp}=    Get BGP Summary    ${UPE1_HANDLE}    default
    
    ${peer_count}=    Get Length    ${bgp}[peers]
    Log    BGP Peers: ${peer_count}    console=yes
    Log    Established: ${bgp}[established]    console=yes
    
    # Log peer details
    Run Keyword If    ${peer_count} > 0
    ...    Log    ✓ BGP peers configured    console=yes
    ...    ELSE
    ...    Log    ⚠ No BGP peers found (may be expected in isolated lab)    WARN

TEST-005: Connect to Aggregation Router SR201
    [Documentation]    Test SSH connection to ASR903 aggregation router
    [Tags]    connection    aggregation
    
    Log    Connecting to SR201 (172.10.1.201)...    console=yes
    
    ${handle}=    Connect To Device
    ...    172.10.1.201
    ...    ASR903
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    Should Not Be Empty    ${handle}
    Log    ✓ Connected successfully, handle: ${handle}    console=yes
    Set Suite Variable    ${SR201_HANDLE}    ${handle}

TEST-006: Execute Command on ASR903
    [Documentation]    Verify command execution on IOS-XE device
    [Tags]    command    aggregation
    
    Log    Executing 'show version' on SR201...    console=yes
    
    ${output}=    Execute Command
    ...    ${SR201_HANDLE}
    ...    show version
    
    # IOS-XE devices show "Cisco IOS Software" or "Cisco IOS-XE Software"
    Should Contain Any    ${output}    Cisco IOS    IOS-XE
    Log    ✓ Command executed successfully    console=yes

TEST-007: Ping Test Between Devices
    [Documentation]    Test connectivity using ping through Go library
    [Tags]    connectivity    ping
    
    Log    Testing ping from UPE1 to SR201...    console=yes
    
    ${result}=    Ping Test
    ...    ${UPE1_HANDLE}
    ...    172.10.1.201
    ...    default
    ...    5
    
    Log    Ping results: ${result}    console=yes
    Log    Success rate: ${result}[success_pct]%    console=yes
    
    # Validate connectivity
    Should Be True    ${result}[success_pct] >= 80
    Log    ✓ Ping test successful    console=yes

TEST-008: Check Interface Status
    [Documentation]    Verify interface operational status
    [Tags]    interface    status
    
    Log    Checking interface status on UPE1...    console=yes
    
    # Check a common interface (adjust based on your lab)
    ${status}=    Get Interface Status
    ...    ${UPE1_HANDLE}
    ...    GigabitEthernet0/0/0/0
    
    Log    Interface status: ${status}    console=yes
    
    Run Keyword If    '${status}[status]' == 'up'
    ...    Log    ✓ Interface is UP    console=yes
    ...    ELSE
    ...    Log    ⚠ Interface is ${status}[status]    WARN

TEST-009: Close All Connections
    [Documentation]    Clean up connections
    [Tags]    cleanup
    
    Log    Closing connections...    console=yes
    
    Close Connection    ${UPE1_HANDLE}
    Close Connection    ${SR201_HANDLE}
    
    Log    ✓ All connections closed    console=yes

*** Keywords ***
Log Test Environment
    Log    ===== POC TEST SUITE STARTING =====    console=yes
    Log    Go Remote Library: localhost:8270    console=yes
    Log    Using Custom GoNetworkLibrary    console=yes
    Log    Username: ${USERNAME}    console=yes
    Log    ==========================================    console=yes
    Log    Available Test Devices:    console=yes
    Log    Core: UPE1, UPE9, UPE21, UPE24    console=yes
    Log    Aggregation: SR201, SR202, SR203, SR204    console=yes
    Log    Switch: Switch1    console=yes
    Log    ==========================================    console=yes
