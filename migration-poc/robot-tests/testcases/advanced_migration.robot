*** Settings ***
Documentation    Advanced Migration Validation Suite
...              This demonstrates more complex testing scenarios
...              for actual migration validation

Library          Remote    http://localhost:8270    WITH NAME    GoLib
Library          Collections
Library          String
Variables        ../data/devices.yaml

Suite Setup      Setup Migration Test
Suite Teardown   Teardown Migration Test

*** Variables ***
${USERNAME}      admin
${PASSWORD}      admin
@{CORE_ROUTERS}  172.10.1.1    172.10.1.9    172.10.1.21    172.10.1.24

*** Test Cases ***
PRE-001: Capture Baseline OSPF State on All Core Routers
    [Documentation]    Capture OSPF state before migration for comparison
    [Tags]    baseline    infrastructure    critical
    
    Log    Capturing baseline OSPF state across all core routers    console=yes
    
    FOR    ${router_ip}    IN    @{CORE_ROUTERS}
        Log    → Connecting to ${router_ip}...    console=yes
        ${handle}=    Connect And Validate    ${router_ip}    ASR9906
        
        ${ospf}=    GoLib.Get OSPF Neighbors    ${handle}
        ${ospf_count}=    Get Length    ${ospf}
        
        Log    ${router_ip}: ${ospf_count} OSPF neighbors    console=yes
        Set Suite Variable    ${BASELINE_OSPF_${router_ip}}    ${ospf_count}
        
        # Validate all neighbors are FULL
        FOR    ${neighbor}    IN    @{ospf}
            Should Be Equal    ${neighbor}[state]    FULL
            ...    msg=Neighbor ${neighbor}[neighbor_id] not in FULL state on ${router_ip}
        END
        
        GoLib.Close Connection    ${handle}
    END
    
    Log    ✓ Baseline OSPF state captured for all core routers    console=yes

PRE-002: Validate BGP Sessions Across All VRFs
    [Documentation]    Ensure all BGP sessions are established before migration
    [Tags]    baseline    services    critical
    
    Log    Validating BGP sessions on core routers    console=yes
    
    # VRFs to check (update based on your environment)
    @{vrfs}=    Create List    default    VPN_ADMS    VPN_SCADA
    
    ${handle}=    Connect And Validate    172.10.1.1    ASR9906
    
    FOR    ${vrf}    IN    @{vrfs}
        Log    Checking VRF: ${vrf}    console=yes
        
        ${bgp}=    GoLib.Get BGP Summary    ${handle}    ${vrf}
        ${established}=    Get From Dictionary    ${bgp}    established
        
        Log    VRF ${vrf}: ${established} established peers    console=yes
        
        # Store for comparison
        Set Suite Variable    ${BASELINE_BGP_${vrf}}    ${established}
        
        # Validate at least some peers are established
        Should Be True    ${established} > 0
        ...    msg=No BGP peers established in VRF ${vrf}
    END
    
    GoLib.Close Connection    ${handle}
    Log    ✓ All BGP sessions validated    console=yes

PRE-003: Baseline Connectivity Matrix
    [Documentation]    Test connectivity between critical endpoints
    [Tags]    baseline    connectivity    critical
    
    Log    Testing connectivity matrix    console=yes
    
    # Define test matrix (source -> destination)
    @{test_matrix}=    Create List
    ...    172.10.1.1|172.10.1.9|default
    ...    172.10.1.1|172.10.1.201|default
    ...    172.10.1.9|172.10.1.202|default
    
    ${results}=    Create Dictionary
    
    FOR    ${test}    IN    @{test_matrix}
        @{parts}=    Split String    ${test}    |
        ${source}=    Get From List    ${parts}    0
        ${dest}=    Get From List    ${parts}    1
        ${vrf}=    Get From List    ${parts}    2
        
        Log    Testing: ${source} → ${dest} (VRF: ${vrf})    console=yes
        
        ${handle}=    Connect And Validate    ${source}    ASR9906
        ${ping_result}=    GoLib.Ping Test    ${handle}    ${dest}    ${vrf}    10
        
        ${success_rate}=    Get From Dictionary    ${ping_result}    success_pct
        Log    Success rate: ${success_rate}%    console=yes
        
        # Store baseline
        ${key}=    Set Variable    ${source}_to_${dest}_${vrf}
        Set To Dictionary    ${results}    ${key}    ${success_rate}
        
        # Validate connectivity
        Should Be True    ${success_rate} >= 80
        ...    msg=Connectivity issue: ${source} → ${dest}
        
        GoLib.Close Connection    ${handle}
    END
    
    Set Suite Variable    ${BASELINE_CONNECTIVITY}    ${results}
    Log    ✓ Connectivity matrix baseline captured    console=yes

POST-001: Compare OSPF State After Migration
    [Documentation]    Compare OSPF state with baseline
    [Tags]    post-check    infrastructure    critical
    
    Log    Comparing post-migration OSPF state with baseline    console=yes
    
    FOR    ${router_ip}    IN    @{CORE_ROUTERS}
        Log    → Validating ${router_ip}...    console=yes
        ${handle}=    Connect And Validate    ${router_ip}    ASR9906
        
        ${ospf}=    GoLib.Get OSPF Neighbors    ${handle}
        ${current_count}=    Get Length    ${ospf}
        ${baseline_count}=    Get Variable Value    ${BASELINE_OSPF_${router_ip}}    0
        
        Log    Baseline: ${baseline_count} neighbors, Current: ${current_count} neighbors    console=yes
        
        # Compare with baseline
        Should Be Equal As Numbers    ${current_count}    ${baseline_count}
        ...    msg=OSPF neighbor count mismatch on ${router_ip}
        
        # Validate all are FULL
        FOR    ${neighbor}    IN    @{ospf}
            Should Be Equal    ${neighbor}[state]    FULL
        END
        
        GoLib.Close Connection    ${handle}
    END
    
    Log    ✓ OSPF state matches baseline    console=yes

POST-002: Validate BGP Recovery After Migration
    [Documentation]    Ensure all BGP sessions re-established
    [Tags]    post-check    services    critical
    
    Log    Validating BGP session recovery    console=yes
    
    @{vrfs}=    Create List    default    VPN_ADMS    VPN_SCADA
    ${handle}=    Connect And Validate    172.10.1.1    ASR9906
    
    FOR    ${vrf}    IN    @{vrfs}
        Log    Checking VRF: ${vrf}    console=yes
        
        # Wait for BGP convergence (up to 5 minutes)
        Wait Until Keyword Succeeds    5 min    30 sec
        ...    BGP Should Be Established    ${handle}    ${vrf}
        
        ${bgp}=    GoLib.Get BGP Summary    ${handle}    ${vrf}
        ${current}=    Get From Dictionary    ${bgp}    established
        ${baseline}=    Get Variable Value    ${BASELINE_BGP_${vrf}}    0
        
        Log    VRF ${vrf}: Baseline=${baseline}, Current=${current}    console=yes
        
        Should Be Equal As Numbers    ${current}    ${baseline}
        ...    msg=BGP peer count mismatch in VRF ${vrf}
    END
    
    GoLib.Close Connection    ${handle}
    Log    ✓ All BGP sessions recovered    console=yes

POST-003: Verify Connectivity Maintained
    [Documentation]    Ensure connectivity maintained after migration
    [Tags]    post-check    connectivity    critical
    
    Log    Verifying connectivity maintained    console=yes
    
    # Re-run connectivity tests and compare with baseline
    FOR    ${key}    IN    @{BASELINE_CONNECTIVITY.keys()}
        @{parts}=    Split String    ${key}    _to_
        ${source}=    Get From List    ${parts}    0
        @{dest_parts}=    Split String    ${parts}[1]    _
        ${dest}=    Get From List    ${dest_parts}    0
        ${vrf}=    Get From List    ${dest_parts}    1
        
        Log    Testing: ${source} → ${dest}    console=yes
        
        ${handle}=    Connect And Validate    ${source}    ASR9906
        ${ping_result}=    GoLib.Ping Test    ${handle}    ${dest}    ${vrf}    10
        ${current_rate}=    Get From Dictionary    ${ping_result}    success_pct
        ${baseline_rate}=    Get From Dictionary    ${BASELINE_CONNECTIVITY}    ${key}
        
        Log    Baseline: ${baseline_rate}%, Current: ${current_rate}%    console=yes
        
        # Allow for small variance (within 5%)
        ${delta}=    Evaluate    abs(${current_rate} - ${baseline_rate})
        Should Be True    ${delta} <= 5
        ...    msg=Connectivity degraded on ${key}
        
        GoLib.Close Connection    ${handle}
    END
    
    Log    ✓ Connectivity maintained within acceptable variance    console=yes

ROLLBACK-001: Validate Rollback Readiness
    [Documentation]    Verify rollback procedures can be executed
    [Tags]    rollback    safety
    
    Log    Validating rollback readiness    console=yes
    
    # Check that we can reach legacy equipment
    ${handle}=    Connect And Validate    172.10.1.1    ASR9906
    ${version}=    GoLib.Execute Command    ${handle}    show version
    Should Contain    ${version}    IOS XR
    GoLib.Close Connection    ${handle}
    
    Log    ✓ Rollback path validated    console=yes

*** Keywords ***
Connect And Validate
    [Arguments]    ${ip}    ${device_type}
    [Documentation]    Connect to device and validate connection
    
    ${handle}=    GoLib.Connect To Device
    ...    ${ip}
    ...    ${device_type}
    ...    ${USERNAME}
    ...    ${PASSWORD}
    
    Should Not Be Empty    ${handle}
    [Return]    ${handle}

BGP Should Be Established
    [Arguments]    ${handle}    ${vrf}
    [Documentation]    Check if BGP sessions are established
    
    ${bgp}=    GoLib.Get BGP Summary    ${handle}    ${vrf}
    ${established}=    Get From Dictionary    ${bgp}    established
    Should Be True    ${established} > 0

Setup Migration Test
    Log    ===== MIGRATION VALIDATION SUITE =====    console=yes
    Log    Test Mode: Advanced Validation    console=yes
    Log    =====================================    console=yes

Teardown Migration Test
    Log    ===== TEST SUITE COMPLETED =====    console=yes
    Log    Review detailed results in log.html    console=yes
    Log    =================================    console=yes
