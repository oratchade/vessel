#!/bin/bash

# Test helper script for db-connector integration tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
TEST_TYPE="${1:-sqlite}"
VERBOSE="${VERBOSE:-false}"
COVERAGE="${COVERAGE:-false}"
TIMEOUT="${TIMEOUT:-120}"

# Function to print colored output
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a port is open
check_port() {
    local port=$1
    local host=${2:-localhost}
    
    if nc -z "$host" "$port" 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Function to wait for service
wait_for_service() {
    local service=$1
    local port=$2
    local max_attempts=30
    local attempt=0
    
    log_info "Waiting for $service to be ready..."
    
    while [ $attempt -lt $max_attempts ]; do
        if check_port "$port"; then
            log_info "$service is ready"
            return 0
        fi
        
        attempt=$((attempt + 1))
        echo "Attempt $attempt/$max_attempts..."
        sleep 2
    done
    
    log_error "$service failed to start within timeout"
    return 1
}

# Function to run tests
run_tests() {
    local test_type=$1
    local test_flags=""
    
    if [ "$VERBOSE" = "true" ]; then
        test_flags="-v"
    fi
    
    if [ "$COVERAGE" = "true" ]; then
        test_flags="$test_flags -cover -coverprofile=coverage.out"
    fi
    
    log_info "Running tests for $test_type..."
    
    case "$test_type" in
        sqlite)
            log_info "Running SQLite integration tests (no external dependencies needed)"
            go test -timeout "${TIMEOUT}s" "$test_flags" ./tests -run "TestIntegration"
            ;;
        mysql)
            log_info "Running MySQL integration tests..."
            wait_for_service "MySQL" 3306
            export DB_TYPE="mysql"
            go test -timeout "${TIMEOUT}s" "$test_flags" ./tests -run "TestIntegration"
            ;;
        postgres)
            log_info "Running PostgreSQL integration tests..."
            wait_for_service "PostgreSQL" 5432
            export DB_TYPE="postgres"
            go test -timeout "${TIMEOUT}s" "$test_flags" ./tests -run "TestIntegration"
            ;;
        mssql)
            log_info "Running MSSQL integration tests..."
            wait_for_service "MSSQL" 1433
            export DB_TYPE="sqlserver"
            go test -timeout "${TIMEOUT}s" "$test_flags" ./tests -run "TestIntegration"
            ;;
        all)
            log_info "Running all integration tests..."
            log_info "Testing SQLite..."
            go test -timeout "${TIMEOUT}s" "$test_flags" ./tests -run "TestIntegration"
            
            if command -v docker &> /dev/null; then
                log_info "Docker found, running database service tests..."
                export DB_TYPE="sqlite"
                log_info "Starting database services..."
                docker-compose -f docker-compose.test.yml up -d
                
                sleep 10
                
                for db in mysql postgres mssql; do
                    log_info "Running tests against $db..."
                    export DB_TYPE="$db"
                    go test -timeout "${TIMEOUT}s" "$test_flags" ./tests -run "TestIntegration" || log_warn "$db tests had issues"
                done
                
                log_info "Stopping database services..."
                docker-compose -f docker-compose.test.yml down
            fi
            ;;
        *)
            log_error "Unknown test type: $test_type"
            echo "Usage: $0 {sqlite|mysql|postgres|mssql|all} [--verbose] [--coverage]"
            exit 1
            ;;
    esac
}

# Function to display coverage report
display_coverage() {
    if [ "$COVERAGE" = "true" ] && [ -f "coverage.out" ]; then
        log_info "Coverage report:"
        go tool cover -func=coverage.out
        
        if command -v go-cover-treemap &> /dev/null; then
            go tool cover -html=coverage.out -o coverage.html
            log_info "HTML coverage report generated: coverage.html"
        fi
    fi
}

# Parse arguments
while [[ $# -gt 1 ]]; do
    case "$2" in
        --verbose)
            VERBOSE="true"
            shift
            ;;
        --coverage)
            COVERAGE="true"
            shift
            ;;
        --timeout)
            TIMEOUT="$3"
            shift 2
            ;;
        *)
            log_error "Unknown option: $2"
            exit 1
            ;;
    esac
done

# Main execution
log_info "Starting db-connector integration tests"
log_info "Test type: $TEST_TYPE"
log_info "Verbose: $VERBOSE"
log_info "Coverage: $COVERAGE"
log_info "Timeout: ${TIMEOUT}s"
echo ""

if run_tests "$TEST_TYPE"; then
    log_info "Tests completed successfully"
    display_coverage
    exit 0
else
    log_error "Tests failed"
    exit 1
fi
