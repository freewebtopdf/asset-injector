#!/bin/bash

# Rollback Script for Asset Injector Microservice
# Rolls back to previous deployment version in Kubernetes
# Supports notification hooks for alerting
# Returns exit code 0 on success, 1 on failure

set -euo pipefail

# Default configuration
DEFAULT_NAMESPACE="default"
DEFAULT_DEPLOYMENT_NAME="asset-injector"
DEFAULT_TIMEOUT=300
DEFAULT_WAIT_FOR_ROLLOUT=true

# Configuration from environment variables or command line
NAMESPACE="${ROLLBACK_NAMESPACE:-$DEFAULT_NAMESPACE}"
DEPLOYMENT_NAME="${ROLLBACK_DEPLOYMENT_NAME:-$DEFAULT_DEPLOYMENT_NAME}"
TIMEOUT="${ROLLBACK_TIMEOUT:-$DEFAULT_TIMEOUT}"
WAIT_FOR_ROLLOUT="${ROLLBACK_WAIT_FOR_ROLLOUT:-$DEFAULT_WAIT_FOR_ROLLOUT}"

# Notification configuration
SLACK_WEBHOOK_URL="${SLACK_WEBHOOK_URL:-}"
DISCORD_WEBHOOK_URL="${DISCORD_WEBHOOK_URL:-}"
EMAIL_RECIPIENTS="${EMAIL_RECIPIENTS:-}"
NOTIFICATION_ENABLED="${NOTIFICATION_ENABLED:-true}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -d|--deployment)
            DEPLOYMENT_NAME="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --no-wait)
            WAIT_FOR_ROLLOUT=false
            shift
            ;;
        --no-notifications)
            NOTIFICATION_ENABLED=false
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -n, --namespace NAMESPACE    Kubernetes namespace (default: $DEFAULT_NAMESPACE)"
            echo "  -d, --deployment NAME        Deployment name (default: $DEFAULT_DEPLOYMENT_NAME)"
            echo "  -t, --timeout SECONDS        Rollout timeout (default: $DEFAULT_TIMEOUT)"
            echo "  --no-wait                    Don't wait for rollout to complete"
            echo "  --no-notifications           Disable notifications"
            echo "  --dry-run                    Show what would be done without executing"
            echo "  --help                       Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  ROLLBACK_NAMESPACE           Same as --namespace"
            echo "  ROLLBACK_DEPLOYMENT_NAME     Same as --deployment"
            echo "  ROLLBACK_TIMEOUT             Same as --timeout"
            echo "  ROLLBACK_WAIT_FOR_ROLLOUT    Wait for rollout (true/false)"
            echo "  SLACK_WEBHOOK_URL            Slack webhook for notifications"
            echo "  DISCORD_WEBHOOK_URL          Discord webhook for notifications"
            echo "  EMAIL_RECIPIENTS             Email addresses for notifications (comma-separated)"
            echo "  NOTIFICATION_ENABLED         Enable notifications (true/false)"
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
if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || [ "$TIMEOUT" -lt 1 ]; then
    echo "Error: Timeout must be a positive number" >&2
    exit 1
fi

# Check if kubectl is available
if ! command -v kubectl >/dev/null 2>&1; then
    echo "Error: kubectl is not available" >&2
    exit 1
fi

# Function to send notifications
send_notification() {
    local message="$1"
    local status="$2"  # success, failure, info
    
    if [ "$NOTIFICATION_ENABLED" != "true" ]; then
        return 0
    fi
    
    local color=""
    local emoji=""
    case "$status" in
        success)
            color="good"
            emoji="✅"
            ;;
        failure)
            color="danger"
            emoji="❌"
            ;;
        info)
            color="warning"
            emoji="ℹ️"
            ;;
    esac
    
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local hostname=$(hostname 2>/dev/null || echo "unknown")
    
    # Send Slack notification
    if [ -n "$SLACK_WEBHOOK_URL" ]; then
        local slack_payload=$(cat <<EOF
{
    "text": "$emoji Asset Injector Rollback",
    "attachments": [
        {
            "color": "$color",
            "fields": [
                {
                    "title": "Message",
                    "value": "$message",
                    "short": false
                },
                {
                    "title": "Deployment",
                    "value": "$DEPLOYMENT_NAME",
                    "short": true
                },
                {
                    "title": "Namespace",
                    "value": "$NAMESPACE",
                    "short": true
                },
                {
                    "title": "Timestamp",
                    "value": "$timestamp",
                    "short": true
                },
                {
                    "title": "Host",
                    "value": "$hostname",
                    "short": true
                }
            ]
        }
    ]
}
EOF
        )
        
        if command -v curl >/dev/null 2>&1; then
            curl -s -X POST -H 'Content-type: application/json' \
                --data "$slack_payload" "$SLACK_WEBHOOK_URL" >/dev/null || true
        fi
    fi
    
    # Send Discord notification
    if [ -n "$DISCORD_WEBHOOK_URL" ]; then
        local discord_color=""
        case "$status" in
            success) discord_color="65280" ;;  # Green
            failure) discord_color="16711680" ;;  # Red
            info) discord_color="16776960" ;;  # Yellow
        esac
        
        local discord_payload=$(cat <<EOF
{
    "embeds": [
        {
            "title": "$emoji Asset Injector Rollback",
            "description": "$message",
            "color": $discord_color,
            "fields": [
                {
                    "name": "Deployment",
                    "value": "$DEPLOYMENT_NAME",
                    "inline": true
                },
                {
                    "name": "Namespace",
                    "value": "$NAMESPACE",
                    "inline": true
                },
                {
                    "name": "Host",
                    "value": "$hostname",
                    "inline": true
                }
            ],
            "timestamp": "$timestamp"
        }
    ]
}
EOF
        )
        
        if command -v curl >/dev/null 2>&1; then
            curl -s -X POST -H 'Content-type: application/json' \
                --data "$discord_payload" "$DISCORD_WEBHOOK_URL" >/dev/null || true
        fi
    fi
    
    # Send email notification (requires mail command)
    if [ -n "$EMAIL_RECIPIENTS" ] && command -v mail >/dev/null 2>&1; then
        local subject="$emoji Asset Injector Rollback - $status"
        local body=$(cat <<EOF
Asset Injector Rollback Notification

Message: $message
Deployment: $DEPLOYMENT_NAME
Namespace: $NAMESPACE
Timestamp: $timestamp
Host: $hostname
EOF
        )
        
        echo "$body" | mail -s "$subject" "$EMAIL_RECIPIENTS" || true
    fi
}

# Function to get current deployment revision
get_current_revision() {
    kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" \
        -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}' 2>/dev/null || echo ""
}

# Function to get previous revision
get_previous_revision() {
    local current_revision="$1"
    if [ -n "$current_revision" ] && [ "$current_revision" -gt 1 ]; then
        echo $((current_revision - 1))
    else
        echo ""
    fi
}

# Function to check if deployment exists
check_deployment_exists() {
    kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" >/dev/null 2>&1
}

# Function to get deployment status
get_deployment_status() {
    kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" \
        -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null || echo "Unknown"
}

# Function to perform the rollback
perform_rollback() {
    local dry_run_flag=""
    if [ "${DRY_RUN:-false}" = "true" ]; then
        dry_run_flag="--dry-run=client"
        echo "DRY RUN MODE - No actual changes will be made"
        echo ""
    fi
    
    echo "Rollback configuration:"
    echo "  Namespace: $NAMESPACE"
    echo "  Deployment: $DEPLOYMENT_NAME"
    echo "  Timeout: ${TIMEOUT}s"
    echo "  Wait for rollout: $WAIT_FOR_ROLLOUT"
    echo "  Notifications: $NOTIFICATION_ENABLED"
    echo ""
    
    # Check if deployment exists
    if ! check_deployment_exists; then
        echo "Error: Deployment '$DEPLOYMENT_NAME' not found in namespace '$NAMESPACE'" >&2
        send_notification "Rollback failed: Deployment '$DEPLOYMENT_NAME' not found in namespace '$NAMESPACE'" "failure"
        return 1
    fi
    
    # Get current revision
    local current_revision
    current_revision=$(get_current_revision)
    if [ -z "$current_revision" ]; then
        echo "Error: Could not determine current deployment revision" >&2
        send_notification "Rollback failed: Could not determine current deployment revision" "failure"
        return 1
    fi
    
    echo "Current deployment revision: $current_revision"
    
    # Get previous revision
    local previous_revision
    previous_revision=$(get_previous_revision "$current_revision")
    if [ -z "$previous_revision" ]; then
        echo "Error: No previous revision available for rollback" >&2
        send_notification "Rollback failed: No previous revision available" "failure"
        return 1
    fi
    
    echo "Rolling back to revision: $previous_revision"
    echo ""
    
    # Send notification about rollback start
    send_notification "Starting rollback from revision $current_revision to revision $previous_revision" "info"
    
    # Perform the rollback
    echo "Executing rollback command..."
    if ! kubectl rollout undo deployment/"$DEPLOYMENT_NAME" -n "$NAMESPACE" $dry_run_flag; then
        echo "Error: Rollback command failed" >&2
        send_notification "Rollback command failed for deployment '$DEPLOYMENT_NAME'" "failure"
        return 1
    fi
    
    if [ "${DRY_RUN:-false}" = "true" ]; then
        echo "DRY RUN: Rollback command would have been executed"
        return 0
    fi
    
    echo "Rollback command executed successfully"
    
    # Wait for rollout to complete if requested
    if [ "$WAIT_FOR_ROLLOUT" = "true" ]; then
        echo "Waiting for rollout to complete (timeout: ${TIMEOUT}s)..."
        
        if kubectl rollout status deployment/"$DEPLOYMENT_NAME" -n "$NAMESPACE" --timeout="${TIMEOUT}s"; then
            echo "Rollout completed successfully"
            
            # Verify deployment is available
            local status
            status=$(get_deployment_status)
            if [ "$status" = "True" ]; then
                echo "Deployment is available and healthy"
                send_notification "Rollback completed successfully. Deployment is healthy." "success"
                return 0
            else
                echo "Warning: Deployment rollback completed but status is not available" >&2
                send_notification "Rollback completed but deployment status is not available" "failure"
                return 1
            fi
        else
            echo "Error: Rollout did not complete within timeout" >&2
            send_notification "Rollback failed: Rollout did not complete within ${TIMEOUT}s timeout" "failure"
            return 1
        fi
    else
        echo "Rollback initiated. Not waiting for completion."
        send_notification "Rollback initiated for deployment '$DEPLOYMENT_NAME'" "info"
        return 0
    fi
}

# Main execution
echo "Starting rollback process..."
echo ""

# Perform the rollback
if perform_rollback; then
    echo ""
    echo "Rollback completed successfully!"
    exit 0
else
    echo ""
    echo "Rollback failed!"
    exit 1
fi