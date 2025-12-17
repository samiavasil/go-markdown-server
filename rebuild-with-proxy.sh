#!/bin/bash
set -e

# Configure your proxy here
PROXY_URL="http://IP:PORT"

echo "======================================"
echo "REBUILD with PROXY - Full Refresh"
echo "======================================"
echo "Using proxy: $PROXY_URL"

echo ""
echo "Step 1: Stopping all containers..."
docker compose down

echo ""
echo "Step 2: Removing old images..."
docker rmi go-markdown-server:latest 2>/dev/null || echo "No old image to remove"

echo ""
echo "Step 3: Building fresh image with proxy..."
docker compose build --no-cache \
  --build-arg HTTP_PROXY=$PROXY_URL \
  --build-arg HTTPS_PROXY=$PROXY_URL \
  --build-arg NO_PROXY="localhost,127.0.0.1,mongo,plantuml" \
  web

echo ""
echo "Step 4: Starting all services..."
docker compose up -d

echo ""
echo "Step 5: Waiting for services to be ready..."
sleep 3

echo ""
echo "Step 6: Checking service status..."
docker compose ps

echo ""
echo "Step 7: Showing recent logs..."
docker logs --tail 15 go-markdown-server

echo ""
echo "======================================"
echo "REBUILD COMPLETE!"
echo "======================================"
echo ""
echo "Access at: http://127.0.0.1:8080"
echo "Hard refresh browser: Ctrl+Shift+R"
echo ""
