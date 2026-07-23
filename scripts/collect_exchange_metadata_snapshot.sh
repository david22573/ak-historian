#!/bin/bash
# collect_exchange_metadata_snapshot.sh
# 
# Collects public exchange metadata and stores it in the archive.
# NEVER calls broker/private APIs. NEVER touches trader/scout.
# 
# Example Usage:
#   ./scripts/collect_exchange_metadata_snapshot.sh

set -e

# Default configuration
EXCHANGE="binance"
MARKET_TYPE="futures_um"
ARCHIVE_ROOT="data/exchange_metadata"

# Ensure output directory exists for logs
mkdir -p runs/logs

LOG_FILE="runs/logs/exchange_metadata_collection_$(date +%Y%m%d_%H%M%S).log"

echo "Starting exchange metadata collection at $(date)" | tee -a "$LOG_FILE"
echo "Target: $EXCHANGE / $MARKET_TYPE" | tee -a "$LOG_FILE"
echo "Archive Root: $ARCHIVE_ROOT" | tee -a "$LOG_FILE"

# 1. Collect Snapshot
echo "Running exchange-metadata-collect..." | tee -a "$LOG_FILE"
./bin/ak-historian exchange-metadata-collect \
    --exchange "$EXCHANGE" \
    --market-type "$MARKET_TYPE" \
    --archive-root "$ARCHIVE_ROOT" \
    --allow-network true \
    --write-raw true \
    --refresh-manifest false >> "$LOG_FILE" 2>&1

# 2. Refresh Manifest
echo "Running exchange-metadata-archive-refresh..." | tee -a "$LOG_FILE"
./bin/ak-historian exchange-metadata-archive-refresh \
    --exchange "$EXCHANGE" \
    --market-type "$MARKET_TYPE" \
    --archive-root "$ARCHIVE_ROOT" >> "$LOG_FILE" 2>&1

# 3. Verify Archive
echo "Running exchange-metadata-archive-verify..." | tee -a "$LOG_FILE"
./bin/ak-historian exchange-metadata-archive-verify \
    --exchange "$EXCHANGE" \
    --market-type "$MARKET_TYPE" \
    --archive-root "$ARCHIVE_ROOT" \
    --strict true >> "$LOG_FILE" 2>&1

echo "Exchange metadata collection completed successfully at $(date)" | tee -a "$LOG_FILE"
