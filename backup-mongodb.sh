#!/bin/bash
set -e

# MongoDB Backup Script
# Creates a timestamped backup of the MongoDB data directory

BACKUP_DIR="./backups"
DATA_DIR="./data/mongodb"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/mongodb-backup-${TIMESTAMP}.tar.gz"

echo "======================================"
echo "MONGODB BACKUP"
echo "======================================"
echo "Data directory: $DATA_DIR"
echo "Backup location: $BACKUP_FILE"
echo ""

# Check if MongoDB data directory exists
if [ ! -d "$DATA_DIR" ]; then
    echo "Error: MongoDB data directory not found: $DATA_DIR"
    echo "Make sure MongoDB container has run at least once."
    exit 1
fi

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Check if MongoDB is running
if docker ps | grep -q "mongo"; then
    echo "WARNING: MongoDB container is running"
    echo "For best results, stop the container first:"
    echo "  docker compose stop mongo"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Backup cancelled."
        exit 0
    fi
fi

echo "Creating backup..."
tar -czf "$BACKUP_FILE" -C ./data mongodb/

if [ $? -eq 0 ]; then
    BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    echo ""
    echo "======================================"
    echo "BACKUP COMPLETE!"
    echo "======================================"
    echo "File: $BACKUP_FILE"
    echo "Size: $BACKUP_SIZE"
    echo ""
    echo "To restore this backup:"
    echo "  ./restore-mongodb.sh $BACKUP_FILE"
else
    echo ""
    echo "Backup failed!"
    exit 1
fi

# Keep only last 5 backups
echo "Cleaning old backups (keeping last 5)..."
ls -t ${BACKUP_DIR}/mongodb-backup-*.tar.gz 2>/dev/null | tail -n +6 | xargs -r rm -v

echo ""
echo "Done!"
