#!/bin/bash

# Import markdown files to go-markdown-server using modern API
# Usage: ./import-posts.sh <directory-with-md-files> [collection-name]

SERVER_URL="http://localhost:8080"
MD_DIR="${1:-.}"
COLLECTION_NAME="${2}"

if [ ! -d "$MD_DIR" ]; then
    echo "Error: Directory $MD_DIR does not exist"
    exit 1
fi

# Auto-detect collection name from directory if not provided
if [ -z "$COLLECTION_NAME" ]; then
    COLLECTION_NAME=$(basename "$MD_DIR")
fi

echo "======================================"
echo "IMPORTING MARKDOWN FILES"
echo "======================================"
echo "Source directory: $MD_DIR"
echo "Collection name: $COLLECTION_NAME"
echo ""

# Count markdown files
file_count=$(find "$MD_DIR" -name "*.md" -type f | wc -l)
echo "Found $file_count markdown file(s)"
echo ""

if [ "$file_count" -eq 0 ]; then
    echo "No markdown files found in $MD_DIR"
    exit 1
fi

# Build curl command with all files
echo "Uploading files to collection '$COLLECTION_NAME'..."
curl_cmd="curl -s -X POST \"${SERVER_URL}/api/collection/create\" -F \"name=${COLLECTION_NAME}\""

# Add each markdown file
while IFS= read -r file; do
    curl_cmd="$curl_cmd -F \"files=@$file\""
done < <(find "$MD_DIR" -name "*.md" -type f)

# Execute the curl command
response=$(eval $curl_cmd)

# Check response
if echo "$response" | grep -q "\"status\":\"success\""; then
    echo ""
    echo "======================================"
    echo "IMPORT COMPLETE!"
    echo "======================================"
    echo "Collection: $COLLECTION_NAME"
    echo "Files imported: $file_count"
    echo ""
    echo "🌐 View at: ${SERVER_URL}/collection/$COLLECTION_NAME"
else
    echo ""
    echo "======================================"
    echo "IMPORT FAILED"
    echo "======================================"
    echo "Response: $response"
    exit 1
fi
