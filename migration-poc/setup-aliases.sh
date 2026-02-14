#!/bin/bash
# Setup Convenient Aliases for MERALCO Testing
# Usage: source setup-aliases.sh

# Set base directory
ROBOT_DIR="/home/cisco/Pre_Post/network-automation-lab/migration-poc/robot-tests"

# Create aliases
alias run-baseline='cd $ROBOT_DIR && robot --pythonpath libraries --outputdir baseline --name "Pre-Migration Baseline" testcases/test_pre_migration_baseline_FINAL.robot'

alias run-monitoring='cd $ROBOT_DIR && robot --pythonpath libraries --outputdir reports --name "Migration Monitoring" testcases/test_during_migration_monitoring.robot'

alias run-vrf='cd $ROBOT_DIR && robot --pythonpath libraries --outputdir reports --name "VRF Validation" testcases/test_vrf_validation.robot'

alias view-report='firefox $ROBOT_DIR/baseline/report.html &'

alias view-log='firefox $ROBOT_DIR/baseline/log.html &'

alias check-server='cd /home/cisco/Pre_Post/network-automation-lab/migration-poc && ./server.sh status'

alias start-server='cd /home/cisco/Pre_Post/network-automation-lab/migration-poc && ./server.sh start'

alias stop-server='cd /home/cisco/Pre_Post/network-automation-lab/migration-poc && ./server.sh stop'

alias restart-server='cd /home/cisco/Pre_Post/network-automation-lab/migration-poc && ./server.sh restart'

# Print available aliases
echo "=========================================="
echo "MERALCO Testing Aliases Loaded"
echo "=========================================="
echo ""
echo "Available commands:"
echo "  run-baseline     - Run pre-migration baseline tests"
echo "  run-monitoring   - Run during-migration monitoring"
echo "  run-vrf          - Run VRF validation tests"
echo "  view-report      - Open test report in Firefox"
echo "  view-log         - Open test log in Firefox"
echo "  check-server     - Check Go server status"
echo "  start-server     - Start Go server"
echo "  stop-server      - Stop Go server"
echo "  restart-server   - Restart Go server"
echo ""
echo "Example usage:"
echo "  $ run-baseline"
echo "  $ view-report"
echo "=========================================="
