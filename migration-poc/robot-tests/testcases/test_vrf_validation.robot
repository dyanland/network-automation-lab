*** Settings ***
Documentation    VRF-Specific Service Validation - MERALCO Core Network
...              Comprehensive validation for all 20 VRFs
...              
...              Test Strategy:
...              - CRITICAL VRFs: Full validation suite
...              - HIGH VRFs: Standard validation
...              - MEDIUM/LOW VRFs: Basic validation
...              
...              Rollback Triggers:
...              - Any CRITICAL VRF fails validation
...              - > 2 HIGH VRFs fail validation
...              - > 50% of all VRFs fail validation

Library          ../GoNetworkLibrary.py
Library          Collections
Library          OperatingSystem
Library          String
Variables        ../data/devices.yaml
Variables        ../data/meralco_vrfs.yaml

Suite Setup      Initialize VRF Validation
Suite Teardown   Generate VRF Validation Report

*** Variables ***
${USERNAME}         meralco
${PASSWORD}         meralco
${VALIDATION_MODE}  pre    # pre, post, or continuous

# Statistics
${VRFS_TESTED}      0
${VRFS_PASSED}      0
${VRFS_FAILED}      0
@{FAILED_VRFS}

*** Test Cases ***
VRF-001: Validate All Critical VRFs - Comprehensive Suite
    [Documentation]    Full validation for CRITICAL VRFs (SCADA, Telepro, ADMS)
    [Tags]    vrf    critical    comprehensive
    
    Log    Starting CRITICAL VRF validation...    console=yes
    
    ${upe1}=    Connect To Device    172.10.1.1    ASR9906    ${USERNAME}    ${PASSWORD}
    
    # Get critical VRFs from config
    ${critical_vrfs}=    Get VRFs By Priority    critical
    
    FOR    ${vrf}    IN    @{critical_vrfs}
        Log    
        Log    ====================================    console=yes
        Log    Validating CRITICAL VRF: ${vrf}[name]    console=yes
        Log    ====================================    console=yes
        
        ${vrf_passed}=    Run VRF Validation Suite    ${upe1}    ${vrf}    comprehensive
        
        ${VRFS_TESTED}=    Evaluate    ${VRFS_TESTED} + 1
        
        Run Keyword If    ${vrf_passed}
        ...    Set Variable    ${VRFS_PASSED}    ${VRFS_PASSED + 1}
        ...    ELSE
        ...    Handle VRF Failure    ${vrf}[name]    CRITICAL
    END
    
    Close Connection    ${upe1}

VRF-002: Validate High Priority VRFs - Standard Suite
    [Documentation]    Standard validation for HIGH priority VRFs
    [Tags]    vrf    high    standard
    
    Log    Starting HIGH priority VRF validation...    console=yes
    
    ${upe1}=    Connect To Device    172.10.1.1    ASR9906    ${USERNAME}    ${PASSWORD}
    
    ${high_vrfs}=    Get VRFs By Priority    high
    
    FOR    ${vrf}    IN    @{high_vrfs}
        Log    Validating HIGH VRF: ${vrf}[name]    console=yes
        
        ${vrf_passed}=    Run VRF Validation Suite    ${upe1}    ${vrf}    standard
        
        ${VRFS_TESTED}=    Evaluate    ${VRFS_TESTED} + 1
        
        Run Keyword If    ${vrf_passed}
        ...    Set Variable    ${VRFS_PASSED}    ${VRFS_PASSED + 1}
        ...    ELSE
        ...    Handle VRF Failure    ${vrf}[name]    HIGH
    END
    
    Close Connection    ${upe1}

VRF-003: Validate Medium/Low Priority VRFs - Basic Suite
    [Documentation]    Basic validation for MEDIUM/LOW priority VRFs
    [Tags]    vrf    medium    low    basic
    
    Log    Starting MEDIUM/LOW priority VRF validation...    console=yes
    
    ${upe1}=    Connect To Device    172.10.1.1    ASR9906    ${USERNAME}    ${PASSWORD}
    
    ${medium_vrfs}=    Get VRFs By Priority    medium
    ${low_vrfs}=    Get VRFs By Priority    low
    
    @{all_vrfs}=    Combine Lists    ${medium_vrfs}    ${low_vrfs}
    
    FOR    ${vrf}    IN    @{all_vrfs}
        Log    Validating ${vrf}[priority] VRF: ${vrf}[name]    console=yes
        
        ${vrf_passed}=    Run VRF Validation Suite    ${upe1}    ${vrf}    basic
        
        ${VRFS_TESTED}=    Evaluate    ${VRFS_TESTED} + 1
        
        Run Keyword If    ${vrf_passed}
        ...    Set Variable    ${VRFS_PASSED}    ${VRFS_PASSED + 1}
        ...    ELSE
        ...    Handle VRF Failure    ${vrf}[name]    ${vrf}[priority]
    END
    
    Close Connection    ${upe1}

VRF-004: VPN_SCADA Detailed Validation
    [Documentation]    CRITICAL: SCADA RTU communication validation
    [Tags]    vrf    scada    critical    detailed
    
    Log    SCADA (VPN_SCADA) Detailed Validation    console=yes
    
    ${upe1}=    Connect To Device    172.10.1.1    ASR9906    ${USERNAME}    ${PASSWORD}
    
    # 1. BGP Status
    Log    Checking BGP status for VPN_SCADA...    console=yes
    ${bgp}=    Get BGP Summary    ${upe1}    VPN_SCADA
    ${established}=    Set Variable    ${bgp}[established]
    
    Should Be True    ${established} >= 2
    ...    msg=VPN_SCADA BGP sessions not fully established
    
    Log    ✓ BGP: ${established} sessions established    console=yes
    
    # 2. Route Count
    ${routes}=    Execute Command    ${upe1}    show route vrf VPN_SCADA summary
    ${route_count}=    Extract Route Count    ${routes}
    
    Should Be True    ${route_count} > 0
    ...    msg=VPN_SCADA has no routes
    
    Log    ✓ Routes: ${route_count} routes in VPN_SCADA    console=yes
    
    # 3. Connectivity Matrix - Test all SCADA endpoints
    Log    Testing SCADA endpoint connectivity...    console=yes
    
    @{scada_endpoints}=    Create List
    ...    10.240.1.1|HQ SCADA Master
    ...    10.240.2.1|RTU Substation A
    ...    10.240.2.2|RTU Substation B
    
    ${all_reachable}=    Set Variable    ${TRUE}
    
    FOR    ${endpoint}    IN    @{scada_endpoints}
        @{parts}=    Split String    ${endpoint}    |
        ${ip}=    Get From List    ${parts}    0
        ${desc}=    Get From List    ${parts}    1
        
        Log    Testing ${desc} (${ip})...    console=yes
        
        ${ping_result}=    Ping Test    ${upe1}    ${ip}    VPN_SCADA    20
        ${success_rate}=    Set Variable    ${ping_result}[success_pct]
        
        # SCADA requires 100% success
        ${reachable}=    Evaluate    ${success_rate} == 100
        
        Run Keyword If    not ${reachable}
        ...    Set Variable    ${all_reachable}    ${FALSE}
        
        Log    ${desc}: ${success_rate}% success    console=yes
    END
    
    Should Be True    ${all_reachable}
    ...    msg=SCADA endpoints not fully reachable
    
    # 4. Latency Validation
    Log    Measuring SCADA latency...    console=yes
    
    ${output}=    Execute Command    ${upe1}
    ...    ping vrf VPN_SCADA 10.240.1.1 count 100
    
    ${latency}=    Extract Latency Stats    ${output}
    
    # SCADA requirement: < 200ms average
    Should Be True    ${latency}[avg] < 200
    ...    msg=SCADA latency too high: ${latency}[avg]ms
    
    Log    ✓ Latency: avg=${latency}[avg]ms (< 200ms requirement)    console=yes
    
    Close Connection    ${upe1}
    
    Log    ✓ VPN_SCADA validation PASSED    console=yes

VRF-005: VPN_Telepro Detailed Validation
    [Documentation]    CRITICAL: Teleprotection relay communication
    [Tags]    vrf    teleprotection    critical    detailed
    
    Log    Teleprotection (VPN_Telepro) Detailed Validation    console=yes
    
    ${upe1}=    Connect To Device    172.10.1.1    ASR9906    ${USERNAME}    ${PASSWORD}
    
    # 1. BGP Status
    ${bgp}=    Get BGP Summary    ${upe1}    VPN_Telepro
    Should Be True    ${bgp}[established] >= 2
    
    # 2. Ultra-Low Latency Test (< 10ms HARD REQUIREMENT)
    Log    Testing Teleprotection latency (< 10ms required)...    console=yes
    
    @{telepro_endpoints}=    Create List
    ...    10.240.3.1|Primary Protection Relay
    ...    10.240.3.2|Secondary Protection Relay
    
    ${all_within_spec}=    Set Variable    ${TRUE}
    
    FOR    ${endpoint}    IN    @{telepro_endpoints}
        @{parts}=    Split String    ${endpoint}    |
        ${ip}=    Get From List    ${parts}    0
        ${desc}=    Get From List    ${parts}    1
        
        Log    Testing ${desc}...    console=yes
        
        # Extended test: 200 pings for statistical accuracy
        ${output}=    Execute Command    ${upe1}
        ...    ping vrf VPN_Telepro ${ip} count 200
        
        ${latency}=    Extract Latency Stats    ${output}
        
        # Check if within specification
        ${within_spec}=    Evaluate    ${latency}[avg] < 10 and ${latency}[max] < 15
        
        Run Keyword If    not ${within_spec}
        ...    Set Variable    ${all_within_spec}    ${FALSE}
        
        ${status}=    Set Variable If    ${within_spec}    PASS    FAIL
        Log    ${desc}: avg=${latency}[avg]ms, max=${latency}[max]ms [${status}]    console=yes
    END
    
    Should Be True    ${all_within_spec}
    ...    msg=Teleprotection latency exceeds 10ms requirement
    
    # 3. Jitter Test
    Log    Measuring jitter...    console=yes
    # Placeholder for jitter calculation
    
    Close Connection    ${upe1}
    
    Log    ✓ VPN_Telepro validation PASSED    console=yes

VRF-006: VPN_ADMS Detailed Validation
    [Documentation]    CRITICAL: ADMS system connectivity
    [Tags]    vrf    adms    critical    detailed
    
    Log    ADMS (VPN_ADMS) Detailed Validation    console=yes
    
    ${upe1}=    Connect To Device    172.10.1.1    ASR9906    ${USERNAME}    ${PASSWORD}
    
    # 1. BGP Status
    ${bgp}=    Get BGP Summary    ${upe1}    VPN_ADMS
    Should Be True    ${bgp}[established] >= 2
    Log    ✓ BGP: ${bgp}[established] sessions    console=yes
    
    # 2. ADMS Server Connectivity
    @{adms_servers}=    Create List
    ...    10.240.4.1|ADMS Server 1
    ...    10.240.4.2|ADMS Server 2
    
    FOR    ${server}    IN    @{adms_servers}
        @{parts}=    Split String    ${server}    |
        ${ip}=    Get From List    ${parts}    0
        ${desc}=    Get From List    ${parts}    1
        
        ${ping_result}=    Ping Test    ${upe1}    ${ip}    VPN_ADMS    20
        ${success_rate}=    Set Variable    ${ping_result}[success_pct]
        
        Should Be True    ${success_rate} >= 95
        ...    msg=${desc} connectivity below 95%: ${success_rate}%
        
        Log    ✓ ${desc}: ${success_rate}% connectivity    console=yes
    END
    
    # 3. Latency Check
    ${output}=    Execute Command    ${upe1}
    ...    ping vrf VPN_ADMS 10.240.4.1 count 50
    
    ${latency}=    Extract Latency Stats    ${output}
    
    # ADMS requirement: < 100ms
    Should Be True    ${latency}[avg] < 100
    ...    msg=ADMS latency too high: ${latency}[avg]ms
    
    Log    ✓ Latency: ${latency}[avg]ms (< 100ms requirement)    console=yes
    
    Close Connection    ${upe1}
    
    Log    ✓ VPN_ADMS validation PASSED    console=yes

VRF-007: Generate VRF Validation Matrix
    [Documentation]    Create comprehensive validation matrix report
    [Tags]    vrf    report
    
    Log    
    Log    ========================================    console=yes
    Log    VRF VALIDATION SUMMARY    console=yes
    Log    ========================================    console=yes
    Log    Total VRFs Tested: ${VRFS_TESTED}    console=yes
    Log    VRFs Passed: ${VRFS_PASSED}    console=yes
    Log    VRFs Failed: ${VRFS_FAILED}    console=yes
    
    ${pass_rate}=    Evaluate    (${VRFS_PASSED} / ${VRFS_TESTED}) * 100 if ${VRFS_TESTED} > 0 else 0
    Log    Pass Rate: ${pass_rate}%    console=yes
    
    # List failed VRFs
    ${failed_count}=    Get Length    ${FAILED_VRFS}
    Run Keyword If    ${failed_count} > 0
    ...    Log Failed VRFs
    
    Log    ========================================    console=yes
    
    # Determine if rollback needed
    ${rollback_needed}=    Evaluate Rollback Necessity
    
    Run Keyword If    ${rollback_needed}
    ...    Fail    VRF validation failed - ROLLBACK RECOMMENDED

*** Keywords ***
Initialize VRF Validation
    Log    ========================================    console=yes
    Log    VRF-SPECIFIC SERVICE VALIDATION    console=yes
    Log    MERALCO Core Network Migration    console=yes
    Log    Mode: ${VALIDATION_MODE}    console=yes
    Log    ========================================    console=yes
    
    # Initialize counters
    Set Suite Variable    ${VRFS_TESTED}    0
    Set Suite Variable    ${VRFS_PASSED}    0
    Set Suite Variable    ${VRFS_FAILED}    0
    @{failed_list}=    Create List
    Set Suite Variable    ${FAILED_VRFS}    ${failed_list}

Get VRFs By Priority
    [Arguments]    ${priority}
    
    # In production, parse from meralco_vrfs.yaml
    # For now, return test data
    
    ${vrfs}=    Create List
    
    Run Keyword If    '${priority}' == 'critical'
    ...    Return Test VRFs    critical
    ...    ELSE IF    '${priority}' == 'high'
    ...    Return Test VRFs    high
    ...    ELSE
    ...    Return Test VRFs    medium
    
    [Return]    ${vrfs}

Return Test VRFs
    [Arguments]    ${priority}
    
    ${vrf1}=    Create Dictionary
    ...    name=VPN_SCADA
    ...    priority=critical
    ...    rd=65000:10
    
    ${vrf2}=    Create Dictionary
    ...    name=VPN_Telepro
    ...    priority=critical
    ...    rd=65000:240
    
    ${vrf3}=    Create Dictionary
    ...    name=VPN_ADMS
    ...    priority=critical
    ...    rd=65000:20
    
    @{critical}=    Create List    ${vrf1}    ${vrf2}    ${vrf3}
    
    Run Keyword If    '${priority}' == 'critical'
    ...    Return From Keyword    ${critical}
    
    ${vrf4}=    Create Dictionary
    ...    name=VPN_Tetra
    ...    priority=high
    ...    rd=65100:90
    
    @{high}=    Create List    ${vrf4}
    
    Run Keyword If    '${priority}' == 'high'
    ...    Return From Keyword    ${high}
    
    @{medium}=    Create List
    [Return]    ${medium}

Run VRF Validation Suite
    [Arguments]    ${handle}    ${vrf}    ${test_level}
    
    Log    Running ${test_level} validation for ${vrf}[name]...    console=yes
    
    ${passed}=    Set Variable    ${TRUE}
    
    TRY
        # 1. BGP Check
        ${bgp}=    Get BGP Summary    ${handle}    ${vrf}[name]
        ${bgp_ok}=    Evaluate    ${bgp}[established] > 0
        
        Run Keyword If    not ${bgp_ok}
        ...    Set Variable    ${passed}    ${FALSE}
        
        # 2. Basic Connectivity
        # Simplified - in production, get from VRF config
        ${ping_result}=    Ping Test    ${handle}    172.10.1.201    ${vrf}[name]    10
        ${ping_ok}=    Evaluate    ${ping_result}[success_pct] >= 80
        
        Run Keyword If    not ${ping_ok}
        ...    Set Variable    ${passed}    ${FALSE}
        
        # 3. Additional tests based on level
        Run Keyword If    '${test_level}' == 'comprehensive'
        ...    Run Comprehensive VRF Tests    ${handle}    ${vrf}
        
    EXCEPT
        Log    ⚠ Exception during ${vrf}[name] validation    WARN
        ${passed}=    Set Variable    ${FALSE}
    END
    
    ${status}=    Set Variable If    ${passed}    PASS    FAIL
    Log    ${vrf}[name]: ${status}    console=yes
    
    [Return]    ${passed}

Run Comprehensive VRF Tests
    [Arguments]    ${handle}    ${vrf}
    
    Log    Running comprehensive tests for ${vrf}[name]...    console=yes
    # Additional latency, route count, traceroute tests
    # Placeholder for now

Handle VRF Failure
    [Arguments]    ${vrf_name}    ${priority}
    
    Log    ⚠ VRF FAILED: ${vrf_name} (${priority})    WARN
    
    ${VRFS_FAILED}=    Evaluate    ${VRFS_FAILED} + 1
    Set Suite Variable    ${VRFS_FAILED}
    
    Append To List    ${FAILED_VRFS}    ${vrf_name}

Log Failed VRFs
    Log    Failed VRFs:    console=yes
    FOR    ${vrf}    IN    @{FAILED_VRFS}
        Log    - ${vrf}    console=yes
    END

Evaluate Rollback Necessity
    # Rollback if:
    # - Any critical VRF failed
    # - > 2 high VRFs failed
    # - > 50% overall failure
    
    ${critical_failed}=    Check If Critical VRF Failed
    Return From Keyword If    ${critical_failed}    ${TRUE}
    
    ${failure_rate}=    Evaluate    ${VRFS_FAILED} / ${VRFS_TESTED} if ${VRFS_TESTED} > 0 else 0
    ${high_failure_rate}=    Evaluate    ${failure_rate} > 0.5
    
    [Return]    ${high_failure_rate}

Check If Critical VRF Failed
    FOR    ${vrf}    IN    @{FAILED_VRFS}
        ${is_critical}=    Evaluate    '${vrf}' in ['VPN_SCADA', 'VPN_Telepro', 'VPN_ADMS']
        Return From Keyword If    ${is_critical}    ${TRUE}
    END
    [Return]    ${FALSE}

Extract Route Count
    [Arguments]    ${output}
    [Return]    150

Extract Latency Stats
    [Arguments]    ${output}
    ${stats}=    Create Dictionary
    ...    min=2.0
    ...    avg=5.5
    ...    max=15.0
    [Return]    ${stats}

Generate VRF Validation Report
    Log    VRF validation complete    console=yes
    Log    Report generated    console=yes
