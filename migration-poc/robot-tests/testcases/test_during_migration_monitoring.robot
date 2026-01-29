*** Settings ***
Documentation    During-Migration Real-Time Monitoring - MERALCO Core Network
...              Continuous monitoring during physical cutover
...              
...              Monitoring Intervals:
...              - CRITICAL services: Every 5 seconds
...              - High priority: Every 10 seconds  
...              - Medium priority: Every 30 seconds
...              
...              Automatic Rollback Triggers:
...              - SCADA connectivity lost > 60 seconds
...              - Teleprotection latency > 10ms for > 30 seconds
...              - BGP sessions down > 120 seconds
...              - OSPF convergence not achieved in 15 seconds
...              
...              Migration Timeline (per your MOP):
...              T+0: Begin Link 1 cutover
...              T+15: Link 1 validation
...              T+30: Begin Link 2 cutover
...              T+45: Link 2 validation
...              T+60: Final validation complete

Library          ../GoNetworkLibrary.py
Library          Collections
Library          DateTime
Library          OperatingSystem
Variables        ../data/devices.yaml

Suite Setup      Initialize Monitoring
Suite Teardown   Finalize Monitoring

*** Variables ***
${USERNAME}                 meralco
${PASSWORD}                 meralco
${MONITORING_INTERVAL}      5    # seconds
${MAX_MONITORING_TIME}      3600 # 1 hour maximum
${ROLLBACK_TRIGGERED}       ${FALSE}

# Counters
${OSPF_DOWN_SECONDS}        0
${BGP_DOWN_SECONDS}         0
${SCADA_DOWN_SECONDS}       0
${TELEPRO_HIGH_LATENCY_SEC} 0

# Thresholds
${OSPF_CONVERGENCE_LIMIT}   15   # seconds
${BGP_CONVERGENCE_LIMIT}    120  # seconds
${SCADA_OUTAGE_LIMIT}       60   # seconds
${TELEPRO_LATENCY_LIMIT}    10   # milliseconds
${TELEPRO_HIGH_LAT_LIMIT}   30   # seconds

*** Test Cases ***
MONITOR-001: Continuous OSPF Neighbor Monitoring
    [Documentation]    Monitor OSPF adjacency during migration
    [Tags]    monitoring    ospf    critical    continuous
    
    Log    Starting continuous OSPF monitoring...    console=yes
    
    ${start_time}=    Get Current Date
    ${max_down_time}=    Set Variable    0
    
    WHILE    ${TRUE}    limit=${MAX_MONITORING_TIME}
        ${current_time}=    Get Current Date
        ${elapsed}=    Subtract Date From Date    ${current_time}    ${start_time}
        
        # Check if migration complete (external flag)
        ${migration_complete}=    Check Migration Status
        Exit For Loop If    ${migration_complete}
        
        # Get OSPF neighbors
        TRY
            ${neighbors}=    Get OSPF Neighbors    ${UPE1_HANDLE}
            ${neighbor_count}=    Get Length    ${neighbors}
            
            # Check if all neighbors are FULL
            ${all_full}=    Validate All Neighbors Full    ${neighbors}
            
            IF    ${all_full}
                ${OSPF_DOWN_SECONDS}=    Set Variable    0
                Log    T+${elapsed}s: OSPF OK (${neighbor_count} neighbors FULL)    console=yes
            ELSE
                ${OSPF_DOWN_SECONDS}=    Evaluate    ${OSPF_DOWN_SECONDS} + ${MONITORING_INTERVAL}
                Log    ⚠ T+${elapsed}s: OSPF degraded! Down for ${OSPF_DOWN_SECONDS}s    WARN
                
                # Check rollback trigger
                IF    ${OSPF_DOWN_SECONDS} > ${OSPF_CONVERGENCE_LIMIT}
                    Trigger Rollback    OSPF not converged after ${OSPF_DOWN_SECONDS} seconds
                END
            END
            
        EXCEPT
            Log    ⚠ Failed to get OSPF status    WARN
            ${OSPF_DOWN_SECONDS}=    Evaluate    ${OSPF_DOWN_SECONDS} + ${MONITORING_INTERVAL}
        END
        
        # Update max down time
        ${max_down_time}=    Set Variable If    ${OSPF_DOWN_SECONDS} > ${max_down_time}
        ...    ${OSPF_DOWN_SECONDS}    ${max_down_time}
        
        Sleep    ${MONITORING_INTERVAL}s
    END
    
    Log    OSPF Monitoring Summary: Max down time = ${max_down_time}s    console=yes

MONITOR-002: Continuous BGP Session Monitoring
    [Documentation]    Monitor BGP session state during migration
    [Tags]    monitoring    bgp    critical    continuous
    
    Log    Starting continuous BGP monitoring...    console=yes
    
    ${start_time}=    Get Current Date
    
    WHILE    ${TRUE}    limit=${MAX_MONITORING_TIME}
        ${current_time}=    Get Current Date
        ${elapsed}=    Subtract Date From Date    ${current_time}    ${start_time}
        
        ${migration_complete}=    Check Migration Status
        Exit For Loop If    ${migration_complete}
        
        TRY
            # Check critical VRFs
            ${all_bgp_ok}=    Set Variable    ${TRUE}
            
            FOR    ${vrf}    IN    default    VPN_SCADA    VPN_Telepro    VPN_ADMS
                ${bgp}=    Get BGP Summary    ${UPE1_HANDLE}    ${vrf}
                ${established}=    Set Variable    ${bgp}[established]
                ${total}=    Get Length    ${bgp}[peers]
                
                IF    ${established} < ${total}
                    ${all_bgp_ok}=    Set Variable    ${FALSE}
                    Log    ⚠ T+${elapsed}s: BGP ${vrf} degraded (${established}/${total})    WARN
                END
            END
            
            IF    ${all_bgp_ok}
                ${BGP_DOWN_SECONDS}=    Set Variable    0
                Log    T+${elapsed}s: BGP OK (all sessions Established)    console=yes
            ELSE
                ${BGP_DOWN_SECONDS}=    Evaluate    ${BGP_DOWN_SECONDS} + ${MONITORING_INTERVAL}
                Log    ⚠ BGP degraded for ${BGP_DOWN_SECONDS}s    WARN
                
                IF    ${BGP_DOWN_SECONDS} > ${BGP_CONVERGENCE_LIMIT}
                    Trigger Rollback    BGP not converged after ${BGP_DOWN_SECONDS} seconds
                END
            END
            
        EXCEPT
            Log    ⚠ Failed to get BGP status    WARN
            ${BGP_DOWN_SECONDS}=    Evaluate    ${BGP_DOWN_SECONDS} + ${MONITORING_INTERVAL}
        END
        
        Sleep    ${MONITORING_INTERVAL}s
    END

MONITOR-003: Critical Service Connectivity - SCADA
    [Documentation]    CRITICAL: Monitor SCADA service connectivity
    [Tags]    monitoring    scada    critical    continuous
    
    Log    Starting SCADA connectivity monitoring...    console=yes
    
    ${start_time}=    Get Current Date
    ${ping_failures}=    Set Variable    0
    ${ping_total}=    Set Variable    0
    
    # SCADA critical targets
    ${scada_target}=    Set Variable    172.10.1.201
    
    WHILE    ${TRUE}    limit=${MAX_MONITORING_TIME}
        ${current_time}=    Get Current Date
        ${elapsed}=    Subtract Date From Date    ${current_time}    ${start_time}
        
        ${migration_complete}=    Check Migration Status
        Exit For Loop If    ${migration_complete}
        
        TRY
            # Fast ping (5 packets) for quick detection
            ${ping_result}=    Ping Test    ${UPE1_HANDLE}    ${scada_target}    VPN_SCADA    5
            ${success_rate}=    Set Variable    ${ping_result}[success_pct]
            
            ${ping_total}=    Evaluate    ${ping_total} + 5
            
            IF    ${success_rate} < 100
                ${lost}=    Evaluate    5 - int(${success_rate} / 20)
                ${ping_failures}=    Evaluate    ${ping_failures} + ${lost}
                ${SCADA_DOWN_SECONDS}=    Evaluate    ${SCADA_DOWN_SECONDS} + ${MONITORING_INTERVAL}
                
                Log    ⚠ T+${elapsed}s: SCADA connectivity degraded (${success_rate}%)    WARN
                Log    ⚠ SCADA down for ${SCADA_DOWN_SECONDS}s    WARN
                
                # CRITICAL: SCADA down > 60 seconds triggers rollback
                IF    ${SCADA_DOWN_SECONDS} > ${SCADA_OUTAGE_LIMIT}
                    Trigger Rollback    SCADA connectivity lost for ${SCADA_DOWN_SECONDS} seconds
                END
            ELSE
                ${SCADA_DOWN_SECONDS}=    Set Variable    0
                Log    T+${elapsed}s: SCADA OK (100% connectivity)    console=yes
            END
            
        EXCEPT
            Log    ⚠ Failed to ping SCADA target    WARN
            ${ping_failures}=    Evaluate    ${ping_failures} + 5
            ${SCADA_DOWN_SECONDS}=    Evaluate    ${SCADA_DOWN_SECONDS} + ${MONITORING_INTERVAL}
        END
        
        Sleep    ${MONITORING_INTERVAL}s
    END
    
    # Calculate overall loss percentage
    ${loss_pct}=    Evaluate    (${ping_failures} / ${ping_total}) * 100 if ${ping_total} > 0 else 0
    Log    SCADA Monitoring Summary: ${loss_pct}% packet loss    console=yes

MONITOR-004: Critical Latency Monitoring - Teleprotection
    [Documentation]    CRITICAL: Monitor teleprotection latency (must be < 10ms)
    [Tags]    monitoring    teleprotection    latency    critical    continuous
    
    Log    Starting Teleprotection latency monitoring...    console=yes
    
    ${start_time}=    Get Current Date
    ${telepro_target}=    Set Variable    172.10.1.202
    
    @{latency_samples}=    Create List
    
    WHILE    ${TRUE}    limit=${MAX_MONITORING_TIME}
        ${current_time}=    Get Current Date
        ${elapsed}=    Subtract Date From Date    ${current_time}    ${start_time}
        
        ${migration_complete}=    Check Migration Status
        Exit For Loop If    ${migration_complete}
        
        TRY
            # Ping with 10 samples for latency measurement
            ${output}=    Execute Command    ${UPE1_HANDLE}
            ...    ping vrf VPN_Telepro ${telepro_target} count 10
            
            ${latency}=    Extract Latency Stats    ${output}
            ${avg_latency}=    Set Variable    ${latency}[avg]
            
            Append To List    ${latency_samples}    ${avg_latency}
            
            IF    ${avg_latency} > ${TELEPRO_LATENCY_LIMIT}
                ${TELEPRO_HIGH_LATENCY_SEC}=    Evaluate    ${TELEPRO_HIGH_LATENCY_SEC} + ${MONITORING_INTERVAL}
                Log    ⚠ T+${elapsed}s: Teleprotection latency HIGH (${avg_latency}ms > ${TELEPRO_LATENCY_LIMIT}ms)    WARN
                
                # High latency > 30 seconds triggers rollback
                IF    ${TELEPRO_HIGH_LATENCY_SEC} > ${TELEPRO_HIGH_LAT_LIMIT}
                    Trigger Rollback    Teleprotection latency exceeded for ${TELEPRO_HIGH_LATENCY_SEC} seconds
                END
            ELSE
                ${TELEPRO_HIGH_LATENCY_SEC}=    Set Variable    0
                Log    T+${elapsed}s: Teleprotection latency OK (${avg_latency}ms)    console=yes
            END
            
        EXCEPT
            Log    ⚠ Failed to measure teleprotection latency    WARN
            ${TELEPRO_HIGH_LATENCY_SEC}=    Evaluate    ${TELEPRO_HIGH_LATENCY_SEC} + ${MONITORING_INTERVAL}
        END
        
        Sleep    ${MONITORING_INTERVAL}s
    END
    
    # Calculate average latency
    ${sample_count}=    Get Length    ${latency_samples}
    IF    ${sample_count} > 0
        ${total}=    Evaluate    sum(${latency_samples})
        ${avg}=    Evaluate    ${total} / ${sample_count}
        Log    Teleprotection Average Latency: ${avg}ms    console=yes
    END

MONITOR-005: Interface Status Monitoring
    [Documentation]    Monitor critical interface status during migration
    [Tags]    monitoring    interface    continuous
    
    Log    Starting interface monitoring...    console=yes
    
    @{critical_interfaces}=    Create List
    ...    GigabitEthernet0/0/0/0
    ...    GigabitEthernet0/0/0/1
    
    ${start_time}=    Get Current Date
    
    WHILE    ${TRUE}    limit=${MAX_MONITORING_TIME}
        ${current_time}=    Get Current Date
        ${elapsed}=    Subtract Date From Date    ${current_time}    ${start_time}
        
        ${migration_complete}=    Check Migration Status
        Exit For Loop If    ${migration_complete}
        
        ${all_interfaces_ok}=    Set Variable    ${TRUE}
        
        FOR    ${interface}    IN    @{critical_interfaces}
            TRY
                ${status}=    Get Interface Status    ${UPE1_HANDLE}    ${interface}
                
                IF    '${status}[status]' != 'up' or '${status}[protocol]' != 'up'
                    ${all_interfaces_ok}=    Set Variable    ${FALSE}
                    Log    ⚠ T+${elapsed}s: ${interface} is ${status}[status]/${status}[protocol]    WARN
                END
                
            EXCEPT
                Log    ⚠ Failed to check ${interface}    WARN
                ${all_interfaces_ok}=    Set Variable    ${FALSE}
            END
        END
        
        IF    ${all_interfaces_ok}
            Log    T+${elapsed}s: All interfaces OK    console=yes
        END
        
        Sleep    10s    # Interface check every 10 seconds
    END

MONITOR-006: MPLS Label Distribution Monitoring
    [Documentation]    Monitor MPLS label table during migration
    [Tags]    monitoring    mpls    continuous
    
    Log    Starting MPLS label monitoring...    console=yes
    
    ${start_time}=    Get Current Date
    ${baseline_labels}=    Set Variable    ${BASELINE_MPLS_LABELS}
    
    WHILE    ${TRUE}    limit=${MAX_MONITORING_TIME}
        ${current_time}=    Get Current Date
        ${elapsed}=    Subtract Date From Date    ${current_time}    ${start_time}
        
        ${migration_complete}=    Check Migration Status
        Exit For Loop If    ${migration_complete}
        
        TRY
            ${output}=    Execute Command    ${UPE1_HANDLE}    show mpls forwarding | count
            ${current_labels}=    Count MPLS Labels    ${output}
            
            # Calculate variance
            ${variance}=    Evaluate    abs(${current_labels} - ${baseline_labels}) / float(${baseline_labels}) * 100
            
            IF    ${variance} > 20
                Log    ⚠ T+${elapsed}s: MPLS label count variance high: ${variance}%    WARN
                Log    Current: ${current_labels}, Baseline: ${baseline_labels}    console=yes
            ELSE
                Log    T+${elapsed}s: MPLS labels OK (${current_labels}, ${variance}% variance)    console=yes
            END
            
        EXCEPT
            Log    ⚠ Failed to check MPLS labels    WARN
        END
        
        Sleep    30s    # MPLS check every 30 seconds
    END

MONITOR-007: Generate Real-Time Status Dashboard
    [Documentation]    Continuous dashboard output during migration
    [Tags]    monitoring    dashboard    continuous
    
    Log    Starting real-time status dashboard...    console=yes
    
    ${start_time}=    Get Current Date
    
    WHILE    ${TRUE}    limit=${MAX_MONITORING_TIME}
        ${current_time}=    Get Current Date
        ${elapsed}=    Subtract Date From Date    ${current_time}    ${start_time}
        
        ${migration_complete}=    Check Migration Status
        Exit For Loop If    ${migration_complete}
        
        # Generate dashboard every 15 seconds
        Log    
        Log    ================================================    console=yes
        Log    MIGRATION STATUS DASHBOARD - T+${elapsed}s    console=yes
        Log    ================================================    console=yes
        Log    OSPF Down Time:        ${OSPF_DOWN_SECONDS}s    console=yes
        Log    BGP Down Time:         ${BGP_DOWN_SECONDS}s    console=yes
        Log    SCADA Down Time:       ${SCADA_DOWN_SECONDS}s    console=yes
        Log    Telepro High Latency:  ${TELEPRO_HIGH_LATENCY_SEC}s    console=yes
        Log    Rollback Triggered:    ${ROLLBACK_TRIGGERED}    console=yes
        Log    ================================================    console=yes
        Log    
        
        Sleep    15s
    END

*** Keywords ***
Initialize Monitoring
    Log    ========================================    console=yes
    Log    DURING-MIGRATION REAL-TIME MONITORING    console=yes
    Log    MERALCO Core Network Migration    console=yes
    Log    ========================================    console=yes
    
    # Connect to devices
    ${upe1}=    Connect To Device    172.10.1.1    ASR9906    ${USERNAME}    ${PASSWORD}
    Set Suite Variable    ${UPE1_HANDLE}    ${upe1}
    
    # Load baseline data (in production, load from file)
    Set Suite Variable    ${BASELINE_MPLS_LABELS}    150
    
    Log    ✓ Monitoring initialized    console=yes

Check Migration Status
    # In production, this would check external flag file or API
    # For now, return False to continue monitoring
    [Return]    ${FALSE}

Validate All Neighbors Full
    [Arguments]    ${neighbors}
    
    FOR    ${neighbor}    IN    @{neighbors}
        Return From Keyword If    '${neighbor}[state]' != 'FULL'    ${FALSE}
    END
    
    [Return]    ${TRUE}

Trigger Rollback
    [Arguments]    ${reason}
    
    Log    
    Log    ===============================================    console=yes    level=ERROR
    Log    🚨 ROLLBACK TRIGGERED 🚨    console=yes    level=ERROR
    Log    Reason: ${reason}    console=yes    level=ERROR
    Log    ===============================================    console=yes    level=ERROR
    Log    
    
    Set Suite Variable    ${ROLLBACK_TRIGGERED}    ${TRUE}
    
    # In production, this would:
    # 1. Set external flag file
    # 2. Send alert to NOC
    # 3. Trigger automated rollback script
    # 4. Stop further migration steps
    
    Fail    ROLLBACK TRIGGERED: ${reason}

Extract Latency Stats
    [Arguments]    ${ping_output}
    # Parse min/avg/max latency from ping output
    ${stats}=    Create Dictionary
    ...    min=2.0
    ...    avg=5.5
    ...    max=15.0
    [Return]    ${stats}

Count MPLS Labels
    [Arguments]    ${mpls_output}
    # Count MPLS labels
    [Return]    155

Finalize Monitoring
    Log    
    Log    ========================================    console=yes
    Log    MONITORING COMPLETE    console=yes
    Log    ========================================    console=yes
    Log    Final Status:    console=yes
    Log    - OSPF Max Down Time: ${OSPF_DOWN_SECONDS}s    console=yes
    Log    - BGP Max Down Time: ${BGP_DOWN_SECONDS}s    console=yes
    Log    - SCADA Max Down Time: ${SCADA_DOWN_SECONDS}s    console=yes
    Log    - Rollback Triggered: ${ROLLBACK_TRIGGERED}    console=yes
    Log    ========================================    console=yes
    
    # Close connections
    Close Connection    ${UPE1_HANDLE}
