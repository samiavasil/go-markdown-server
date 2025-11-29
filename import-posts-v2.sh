#!/bin/bash

# Import markdown files to go-markdown-server with collection support
# Usage: ./import-posts-v2.sh <directory-with-md-files> [collection-name]

SERVER_URL="http://localhost:8080"
API_KEY="124252"
MD_DIR="${1:-.}"
COLLECTION="${2}"

if [ ! -d "$MD_DIR" ]; then
    echo "Error: Directory $MD_DIR does not exist"
    exit 1
fi

# If no collection specified, use directory name
if [ -z "$COLLECTION" ]; then
    COLLECTION=$(basename "$MD_DIR")
fi

echo "Importing markdown files from: $MD_DIR"
echo "Collection: $COLLECTION"
echo "========================================="

# Find all .md files
find "$MD_DIR" -name "*.md" -type f | while read -r file; do
    # Extract filename without extension for URL
    filename=$(basename "$file" .md)
    
    # Create URL-friendly slug
    url=$(echo "$filename" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd '[:alnum:]-')
    
    # Use filename as title (replace dashes/underscores with spaces)
    title=$(echo "$filename" | tr '_-' ' ')
    
    # Check if this is an index file
    isIndex="false"
    if [[ "$filename" =~ ^[Ii]ndex$ ]] || [[ "$filename" =~ ^[Rr][Ee][Aa][Dd][Mm][Ee]$ ]]; then
        isIndex="true"
    fi
    
    # Read file content and URL encode
    body=$(python3 -c "import urllib.parse; import sys; print(urllib.parse.quote(sys.stdin.read()))" < "$file")
    
    # URL encode collection name
    collection_encoded=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$COLLECTION'))")
    
    # URL encode title
    title_encoded=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$title'))")
    
    echo -n "Importing: $title ($url) [collection=$COLLECTION, index=$isIndex] ... "
    
    # Send to server with collection and isIndex
    response=$(curl -s "${SERVER_URL}/add?title=${title_encoded}&url=${url}&body=${body}&collection=${collection_encoded}&isIndex=${isIndex}&key=${API_KEY}")
    
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
echo "View collection at: ${SERVER_URL}/collection/${COLLECTION}"
