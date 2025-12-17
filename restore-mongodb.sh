#!/bin/bash
set -e

# MongoDB Restore Script
# Restores MongoDB data from a backup file

BACKUP_FILE="$1"
DATA_DIR="./data/mongodb"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: ./restore-mongodb.sh <backup-file.tar.gz>"
    echo ""
    echo "Available backups:"
    ls -lh backups/mongodb-backup-*.tar.gz 2>/dev/null || echo "  No backups found"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: Backup file not found: $BACKUP_FILE"
    exit 1
fi

echo "======================================"
echo "MONGODB RESTORE"
echo "======================================"
echo "Backup file: $BACKUP_FILE"
echo "Target directory: $DATA_DIR"
echo ""

# Check if MongoDB is running
if docker ps | grep -q "mongo"; then
    echo "Error: MongoDB container is running!"
    echo "Stop it first:"
    echo "  docker compose stop mongo"
    exit 1
fi

# Warn about data loss
echo "WARNING: This will DELETE all current MongoDB data!"
read -p "Are you sure you want to continue? (yes/N) " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Restore cancelled."
    exit 0
fi

echo "Removing current data..."
rm -rf "$DATA_DIR"/*

echo "Extracting backup..."
tar -xzf "$BACKUP_FILE" -C ./data/

if [ $? -eq 0 ]; then
    echo ""
    echo "======================================"
    echo "RESTORE COMPLETE!"
    echo "======================================"
    echo ""
    echo "Start MongoDB container:"
    echo "  docker compose up -d mongo"
else
    echo ""
    echo "Restore failed!"
    exit 1
fi
