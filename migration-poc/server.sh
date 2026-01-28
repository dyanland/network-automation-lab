#!/bin/bash

# Server management script for Go Remote Library

PROJECT_ROOT="/home/cisco/Pre_Post/network-automation-lab/migration-poc"
BINARY_PATH="${PROJECT_ROOT}/build/network-library-linux-amd64"
PORT=8270

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

case "$1" in
    start)
        echo "Starting Go Remote Library..."
        
        # Check if already running
        if lsof -i :${PORT} > /dev/null 2>&1; then
            echo -e "${YELLOW}⚠${NC} Server already running on port ${PORT}"
            echo "Use '$0 restart' to restart the server"
            exit 1
        fi
        
        # Check if binary exists
        if [ ! -f "${BINARY_PATH}" ]; then
            echo -e "${RED}✗${NC} Binary not found at: ${BINARY_PATH}"
            echo "Please run: ./build.sh"
            exit 1
        fi
        
        # Make sure it's executable
        chmod +x "${BINARY_PATH}"
        
        # Start server in background
        cd "${PROJECT_ROOT}"
        nohup "${BINARY_PATH}" > /tmp/go-library.log 2>&1 &
        
        # Wait for startup
        sleep 2
        
        # Check if started successfully
        if lsof -i :${PORT} > /dev/null 2>&1; then
            PID=$(lsof -ti :${PORT})
            echo -e "${GREEN}✓${NC} Server started successfully (PID: ${PID})"
            echo "Listening on port ${PORT}"
            echo "Log file: /tmp/go-library.log"
        else
            echo -e "${RED}✗${NC} Server failed to start"
            echo "Check log: cat /tmp/go-library.log"
            exit 1
        fi
        ;;
        
    stop)
        echo "Stopping Go Remote Library..."
        
        if lsof -i :${PORT} > /dev/null 2>&1; then
            PID=$(lsof -ti :${PORT})
            kill ${PID}
            sleep 1
            
            # Force kill if still running
            if lsof -i :${PORT} > /dev/null 2>&1; then
                kill -9 ${PID}
                echo -e "${YELLOW}⚠${NC} Server force killed"
            else
                echo -e "${GREEN}✓${NC} Server stopped"
            fi
        else
            echo -e "${YELLOW}⚠${NC} Server not running"
        fi
        ;;
        
    restart)
        echo "Restarting Go Remote Library..."
        $0 stop
        sleep 1
        $0 start
        ;;
        
    status)
        echo "Checking Go Remote Library status..."
        echo ""
        
        if lsof -i :${PORT} > /dev/null 2>&1; then
            PID=$(lsof -ti :${PORT})
            echo -e "${GREEN}✓${NC} Server is RUNNING"
            echo "  PID: ${PID}"
            echo "  Port: ${PORT}"
            echo "  Binary: ${BINARY_PATH}"
            echo ""
            echo "Process details:"
            ps aux | grep ${PID} | grep -v grep
            echo ""
            echo "Network:"
            netstat -an | grep ${PORT}
        else
            echo -e "${RED}✗${NC} Server is NOT running"
            echo ""
            echo "Start with: $0 start"
        fi
        ;;
        
    log)
        echo "Showing server log (last 50 lines)..."
        echo "========================================"
        tail -50 /tmp/go-library.log
        ;;
        
    *)
        echo "Usage: $0 {start|stop|restart|status|log}"
        echo ""
        echo "Commands:"
        echo "  start    - Start the Go server"
        echo "  stop     - Stop the Go server"
        echo "  restart  - Restart the Go server"
        echo "  status   - Check server status"
        echo "  log      - View server log"
        echo ""
        echo "Example:"
        echo "  $0 start"
        echo "  $0 status"
        exit 1
        ;;
esac

exit 0
