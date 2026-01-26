#!/bin/bash
# test_iosxr.sh - Test IOS-XR command execution

HOST="172.10.1.1"
USER="meralco"
PASS="meralco"

COMMANDS=(
    "show version"
    "show platform"
    "show interfaces brief"
)

for CMD in "${COMMANDS[@]}"; do
    echo "=== $CMD ==="
    sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR "$USER@$HOST" "$CMD"
    echo ""
done