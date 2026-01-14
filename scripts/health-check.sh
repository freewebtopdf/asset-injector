#!/bin/bash

# Health Check Script for Asset Injector Microservice
# Verifies deployment health by checking /health endpoint
# Supports configurable timeout and retry logic
# Returns exit code 0 on success, 1 on failure

set -euo pipefail

# Default configuration
DEFAULT_HOST="localhost"
DEFAULT_PORT="8080"
DEFAULT_PATH="/health"
DEFAULT_TIMEOUT=30
DEFAULT_RETRY_COUNT=3
DEFAULT_RETRY_DELAY=5
DEFAULT_EXPECTED_STATUS=200

# Configuration from environment variables or command line
HOST="${HEALTH_CHECK_HOST:-$DEFAULT_HOST}"
PORT="${HEALTH_CHECK_PORT:-$DEFAULT_PORT}"
PATH_ENDPOINT="${HEALTH_CHECK_PATH:-$DEFAULT_PATH}"
TIMEOUT="${HEALTH_CHECK_TIMEOUT:-$DEFAULT_TIMEOUT}"
RETRY_COUNT="${HEALTH_CHECK_RETRY_COUNT:-$DEFAULT_RETRY_COUNT}"
RETRY_DELAY="${HEALTH_CHECK_RETRY_DELAY:-$DEFAULT_RETRY_DELAY}"
EXPECTED_STATUS="${HEALTH_CHECK_EXPECTED_STATUS:-$DEFAULT_EXPECTED_STATUS}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)
            HOST="$2"
            shift 2
            ;;
        -p|--port)
            PORT="$2"
            shift 2
            ;;
        --path)
            PATH_ENDPOINT="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -r|--retry-count)
            RETRY_COUNT="$2"
            shift 2
            ;;
        -d|--retry-delay)
            RETRY_DELAY="$2"
            shift 2
            ;;
        -s|--expected-status)
            EXPECTED_STATUS="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -h, --host HOST              Target host (default: $DEFAULT_HOST)"
            echo "  -p, --port PORT              Target port (default: $DEFAULT_PORT)"
            echo "  --path PATH                  Health check path (default: $DEFAULT_PATH)"
            echo "  -t, --timeout SECONDS        Request timeout (default: $DEFAULT_TIMEOUT)"
            echo "  -r, --retry-count COUNT      Number of retries (default: $DEFAULT_RETRY_COUNT)"
            echo "  -d, --retry-delay SECONDS    Delay between retries (default: $DEFAULT_RETRY_DELAY)"
            echo "  -s, --expected-status CODE   Expected HTTP status code (default: $DEFAULT_EXPECTED_STATUS)"
            echo "  --help                       Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  HEALTH_CHECK_HOST            Same as --host"
            echo "  HEALTH_CHECK_PORT            Same as --port"
            echo "  HEALTH_CHECK_PATH            Same as --path"
            echo "  HEALTH_CHECK_TIMEOUT         Same as --timeout"
            echo "  HEALTH_CHECK_RETRY_COUNT     Same as --retry-count"
            echo "  HEALTH_CHECK_RETRY_DELAY     Same as --retry-delay"
            echo "  HEALTH_CHECK_EXPECTED_STATUS Same as --expected-status"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Use --help for usage information" >&2
            exit 1
            ;;
    esac
done

# Validate numeric parameters
if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
    echo "Error: Port must be a number between 1 and 65535" >&2
    exit 1
fi

if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || [ "$TIMEOUT" -lt 1 ]; then
    echo "Error: Timeout must be a positive number" >&2
    exit 1
fi

if ! [[ "$RETRY_COUNT" =~ ^[0-9]+$ ]] || [ "$RETRY_COUNT" -lt 1 ]; then
    echo "Error: Retry count must be a positive number" >&2
    exit 1
fi

if ! [[ "$RETRY_DELAY" =~ ^[0-9]+$ ]] || [ "$RETRY_DELAY" -lt 0 ]; then
    echo "Error: Retry delay must be a non-negative number" >&2
    exit 1
fi

if ! [[ "$EXPECTED_STATUS" =~ ^[0-9]+$ ]] || [ "$EXPECTED_STATUS" -lt 100 ] || [ "$EXPECTED_STATUS" -gt 599 ]; then
    echo "Error: Expected status must be a valid HTTP status code (100-599)" >&2
    exit 1
fi

# Build the health check URL
URL="http://${HOST}:${PORT}${PATH_ENDPOINT}"

echo "Health check configuration:"
echo "  URL: $URL"
echo "  Timeout: ${TIMEOUT}s"
echo "  Retry count: $RETRY_COUNT"
echo "  Retry delay: ${RETRY_DELAY}s"
echo "  Expected status: $EXPECTED_STATUS"
echo ""

# Function to perform a single health check
perform_health_check() {
    local attempt=$1
    echo "Attempt $attempt/$RETRY_COUNT: Checking $URL"
    
    # Use curl with timeout and capture both status code and response
    local http_code
    local response
    local curl_exit_code
    
    # Capture curl output and exit code
    if command -v curl >/dev/null 2>&1; then
        response=$(curl -s -w "%{http_code}" --connect-timeout "$TIMEOUT" --max-time "$TIMEOUT" "$URL" 2>/dev/null || echo "000")
        curl_exit_code=$?
        http_code="${response: -3}"  # Last 3 characters are the HTTP status code
        response="${response%???}"   # Remove last 3 characters to get response body
    else
        echo "Error: curl is not available" >&2
        return 1
    fi
    
    # Check curl exit code first
    if [ $curl_exit_code -ne 0 ]; then
        echo "  Connection failed (curl exit code: $curl_exit_code)"
        return 1
    fi
    
    # Validate HTTP status code format
    if ! [[ "$http_code" =~ ^[0-9]{3}$ ]]; then
        echo "  Invalid HTTP status code received: $http_code"
        return 1
    fi
    
    echo "  HTTP Status: $http_code"
    
    # Check if status code matches expected
    if [ "$http_code" -eq "$EXPECTED_STATUS" ]; then
        echo "  Health check passed!"
        
        # If response body is not empty, show it (truncated)
        if [ -n "$response" ] && [ "$response" != "null" ]; then
            echo "  Response: $(echo "$response" | head -c 200)"
            if [ ${#response} -gt 200 ]; then
                echo "..."
            fi
        fi
        
        return 0
    else
        echo "  Health check failed: Expected status $EXPECTED_STATUS, got $http_code"
        
        # Show response body for debugging (truncated)
        if [ -n "$response" ] && [ "$response" != "null" ]; then
            echo "  Response: $(echo "$response" | head -c 200)"
            if [ ${#response} -gt 200 ]; then
                echo "..."
            fi
        fi
        
        return 1
    fi
}

# Main health check loop with retry logic
echo "Starting health check..."
for attempt in $(seq 1 "$RETRY_COUNT"); do
    if perform_health_check "$attempt"; then
        echo ""
        echo "Health check successful!"
        exit 0
    fi
    
    # If this wasn't the last attempt, wait before retrying
    if [ "$attempt" -lt "$RETRY_COUNT" ]; then
        echo "  Waiting ${RETRY_DELAY}s before retry..."
        sleep "$RETRY_DELAY"
        echo ""
    fi
done

echo ""
echo "Health check failed after $RETRY_COUNT attempts"
exit 1