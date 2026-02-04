#!/bin/bash
#===============================================================================
# MERALCO Network Health Check Logger - Wrapper Script
# Simplifies execution with common use cases
#===============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXECUTABLE="$SCRIPT_DIR/ssh_health_check"
DEFAULT_WORKERS=5

# Check if executable exists
if [ ! -f "$EXECUTABLE" ]; then
    # Try platform-specific binary
    if [ -f "$SCRIPT_DIR/ssh_health_check_linux_amd64" ]; then
        EXECUTABLE="$SCRIPT_DIR/ssh_health_check_linux_amd64"
    else
        echo -e "${RED}ERROR: Executable not found. Please build first.${NC}"
        exit 1
    fi
fi

# Function to display banner
show_banner() {
    echo -e "${BLUE}"
    echo "================================================================================"
    echo "  MERALCO Network Health Check Logger"
    echo "  Core Migration: ASR9010 -> ASR9906"
    echo "================================================================================"
    echo -e "${NC}"
}

# Function to display usage
usage() {
    show_banner
    echo "Usage: $0 <mode> [options]"
    echo ""
    echo "Modes:"
    echo "  pre-migration     Collect baseline before migration"
    echo "  post-migration    Validate after migration completion"
    echo "  quick             Fast health check (minimal commands)"
    echo "  full              Full health check (all commands)"
    echo "  custom            Run with custom command file"
    echo ""
    echo "Options:"
    echo "  -u, --username    SSH username (required)"
    echo "  -p, --password    SSH password (required)"
    echo "  -t, --targets     Target file (default: target.txt)"
    echo "  -w, --workers     Number of parallel workers (default: 5)"
    echo "  -v, --verbose     Enable verbose output"
    echo "  -d, --dry-run     Dry run (no actual connections)"
    echo "  -h, --help        Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 pre-migration -u admin -p MyPassword"
    echo "  $0 quick -u admin -p MyPassword -v"
    echo "  $0 custom -u admin -p MyPassword -c my_commands.txt"
    echo ""
}

# Parse arguments
MODE=""
USERNAME=""
PASSWORD=""
TARGETS="target.txt"
WORKERS=$DEFAULT_WORKERS
VERBOSE=""
DRYRUN=""
CUSTOM_CMD=""

# Get mode
if [ $# -lt 1 ]; then
    usage
    exit 1
fi

MODE=$1
shift

# Parse remaining arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--username)
            USERNAME="$2"
            shift 2
            ;;
        -p|--password)
            PASSWORD="$2"
            shift 2
            ;;
        -t|--targets)
            TARGETS="$2"
            shift 2
            ;;
        -w|--workers)
            WORKERS="$2"
            shift 2
            ;;
        -c|--commands)
            CUSTOM_CMD="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -d|--dry-run)
            DRYRUN="-dry-run"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

# Validate required arguments
if [ -z "$USERNAME" ] || [ -z "$PASSWORD" ]; then
    echo -e "${RED}ERROR: Username and password are required${NC}"
    usage
    exit 1
fi

# Set command file based on mode
case $MODE in
    pre-migration)
        CMD_FILE="command.txt"
        PHASE="pre-migration"
        echo -e "${YELLOW}Running PRE-MIGRATION baseline collection...${NC}"
        ;;
    post-migration)
        CMD_FILE="command_post.txt"
        PHASE="post-migration"
        echo -e "${YELLOW}Running POST-MIGRATION validation...${NC}"
        ;;
    quick)
        CMD_FILE="command_quick.txt"
        PHASE="health-check"
        echo -e "${YELLOW}Running QUICK health check...${NC}"
        ;;
    full)
        CMD_FILE="command.txt"
        PHASE="health-check"
        echo -e "${YELLOW}Running FULL health check...${NC}"
        ;;
    custom)
        if [ -z "$CUSTOM_CMD" ]; then
            echo -e "${RED}ERROR: Custom mode requires -c/--commands option${NC}"
            exit 1
        fi
        CMD_FILE="$CUSTOM_CMD"
        PHASE="health-check"
        echo -e "${YELLOW}Running CUSTOM health check with $CMD_FILE...${NC}"
        ;;
    *)
        echo -e "${RED}ERROR: Unknown mode: $MODE${NC}"
        usage
        exit 1
        ;;
esac

# Check if command file exists
if [ ! -f "$SCRIPT_DIR/$CMD_FILE" ] && [ ! -f "$CMD_FILE" ]; then
    echo -e "${RED}ERROR: Command file not found: $CMD_FILE${NC}"
    exit 1
fi

# Check if target file exists
if [ ! -f "$SCRIPT_DIR/$TARGETS" ] && [ ! -f "$TARGETS" ]; then
    echo -e "${RED}ERROR: Target file not found: $TARGETS${NC}"
    exit 1
fi

# Build the command
show_banner
echo -e "${GREEN}Configuration:${NC}"
echo "  Mode:       $MODE"
echo "  Phase:      $PHASE"
echo "  Commands:   $CMD_FILE"
echo "  Targets:    $TARGETS"
echo "  Workers:    $WORKERS"
echo "  Verbose:    ${VERBOSE:-no}"
echo "  Dry Run:    ${DRYRUN:-no}"
echo ""

# Confirm execution
if [ -z "$DRYRUN" ]; then
    echo -e "${YELLOW}Press Enter to start or Ctrl+C to cancel...${NC}"
    read
fi

# Execute
cd "$SCRIPT_DIR"
$EXECUTABLE \
    -u "$USERNAME" \
    -p "$PASSWORD" \
    -c "$CMD_FILE" \
    -t "$TARGETS" \
    -phase "$PHASE" \
    -w "$WORKERS" \
    $VERBOSE \
    $DRYRUN

EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo ""
    echo -e "${GREEN}================================================================================${NC}"
    echo -e "${GREEN}  Health check completed successfully!${NC}"
    echo -e "${GREEN}  Check output directory for results.${NC}"
    echo -e "${GREEN}================================================================================${NC}"
else
    echo ""
    echo -e "${RED}================================================================================${NC}"
    echo -e "${RED}  Health check completed with errors. Exit code: $EXIT_CODE${NC}"
    echo -e "${RED}  Review the output for details.${NC}"
    echo -e "${RED}================================================================================${NC}"
fi

exit $EXIT_CODE
