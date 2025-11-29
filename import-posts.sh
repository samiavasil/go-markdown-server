#!/bin/bash

# Import markdown files to go-markdown-server
# Usage: ./import-posts.sh <directory-with-md-files>

SERVER_URL="http://localhost:8080"
API_KEY="124252"
MD_DIR="${1:-.}"

if [ ! -d "$MD_DIR" ]; then
    echo "Error: Directory $MD_DIR does not exist"
    exit 1
fi

echo "Importing markdown files from: $MD_DIR"
echo "========================================="

# Find all .md files
find "$MD_DIR" -name "*.md" -type f | while read -r file; do
    # Extract filename without extension for URL
    filename=$(basename "$file" .md)
    
    # Create URL-friendly slug
    url=$(echo "$filename" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd '[:alnum:]-')
    
    # Use filename as title (replace dashes/underscores with spaces)
    title=$(echo "$filename" | tr '_-' ' ')
    
    # Read file content and URL encode
    body=$(python3 -c "import urllib.parse; import sys; print(urllib.parse.quote(sys.stdin.read()))" < "$file")
    
    echo -n "Importing: $title ($url) ... "
    
    # Send to server
    response=$(curl -s "${SERVER_URL}/add?title=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$title'))")&url=${url}&body=${body}&key=${API_KEY}")
    
    if [ "$response" = "success" ]; then
        echo "✓ Success"
    else
        echo "✗ Failed: $response"
    fi
    
    # Small delay to avoid overwhelming the server
    sleep 0.1
done

echo "========================================="
echo "Import complete!"
echo "View posts at: ${SERVER_URL}/"
