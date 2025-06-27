#!/bin/bash

STACK=${1:-duracloud-lyrasis}
BUCKET=${2}
OBJECT=${3}

TABLE_NAME="${STACK}-checksum-scheduler-table"

# Calculate TTL for 1 day ago (in Unix timestamp)
EXPIRED_TTL=$(date -d "1 day ago" +%s)

if [ -n "$BUCKET" ] && [ -n "$OBJECT" ]; then
    echo "Creating scheduler: $BUCKET/$OBJECT"
    aws dynamodb put-item \
        --table-name "$TABLE_NAME" \
        --item "{
            \"BucketName\": {\"S\": \"$BUCKET\"},
            \"ObjectKey\": {\"S\": \"$OBJECT\"},
            \"NextChecksumDate\": {\"S\": \"$(date -d '1 day ago' --iso-8601=seconds)\"},
            \"TTL\": {\"N\": \"$EXPIRED_TTL\"}
        }" \
        --return-values NONE
else
    echo "Usage: $0 [stack-name] <bucket-name> <object-key>"
    echo "Example: $0 duracloud-lyrasis my-bucket path/to/file.txt"
    exit 1
fi

echo "Item created/updated with expired TTL. DynamoDB will process it within ~minutes (max 48 hours)."
echo "This will (eventually) trigger the checksum verification function."
