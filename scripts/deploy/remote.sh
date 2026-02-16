#!/bin/bash
# Remote Deployment Script for Flywall
#
# Usage: ./deploy-remote.sh <user@host> [local_binary]
#
# Process:
# 1. Calculates checksum of local binary
# 2. SCPs binary to /usr/sbin/flywall_new on remote
# 3. Triggers zero-downtime upgrade via `flywall ctl upgrade`
# 4. If successful, finalizes install by moving binary to /usr/sbin/flywall (and backing up old)

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

if [ $# -lt 1 ]; then
    echo "Usage: $0 <user@host> [local_binary]"
    exit 1
fi

REMOTE_HOST="$1"
LOCAL_BINARY="${2:-build/flywall}"

if [ ! -f "$LOCAL_BINARY" ]; then
    log_error "Binary not found at $LOCAL_BINARY. Run 'make build-linux' first?"
    exit 1
fi

log_info "Deploying to $REMOTE_HOST..."

# 1. Calculate Checksum
log_info "Calculating checksum..."
if command -v sha256sum >/dev/null; then
    CHECKSUM=$(sha256sum "$LOCAL_BINARY" | awk '{print $1}')
else
    CHECKSUM=$(shasum -a 256 "$LOCAL_BINARY" | awk '{print $1}')
fi
log_info "Checksum: $CHECKSUM"

# 2. Upload Binary
log_info "Uploading binary..."
scp "$LOCAL_BINARY" "${REMOTE_HOST}:/usr/sbin/flywall_new"
# Ensure executable
ssh "$REMOTE_HOST" "chmod +x /usr/sbin/flywall_new"

# 3. Trigger Upgrade
log_info "Triggering zero-downtime upgrade..."
# We run this in a way that captures output. If it fails, the script exits.
# We also expect the remote to have `flywall` in path or use full path. Assuming /usr/sbin is in path.
if ssh "$REMOTE_HOST" "/usr/sbin/flywall ctl upgrade --checksum $CHECKSUM"; then
    log_ok "Upgrade successful!"

    # 4. Finalize Installation (Swap binaries)
    log_info "Finalizing installation (persisting binary)..."
    ssh "$REMOTE_HOST" "
        if [ -f /usr/sbin/flywall ]; then
            mv /usr/sbin/flywall /usr/sbin/flywall_old
        fi
        mv /usr/sbin/flywall_new /usr/sbin/flywall
        echo 'Binary swapped. Backup at /usr/sbin/flywall_old'
    "
    log_ok "Deployment complete."
else
    log_error "Upgrade failed. The new binary is still at /usr/sbin/flywall_new"
    log_error "Check remote logs: ssh $REMOTE_HOST 'cat /opt/flywall/var/log/flywall.log'"
    exit 1
fi
