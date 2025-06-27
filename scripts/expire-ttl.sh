#!/bin/bash

STACK=${1:-duracloud-lyrasis}
BUCKET=${2}
OBJECT=${3}

TABLE_NAME="${STACK}-checksum-scheduler-table"

# Calculate TTL for 1 day ago (in Unix timestamp)
EXPIRED_TTL=$(date -d "1 day ago" +%s)

if [ -n "$BUCKET" ] && [ -n "$OBJECT" ]; then
    echo "Expiring TTL for item: $BUCKET/$OBJECT"
    aws dynamodb update-item \
        --table-name "$TABLE_NAME" \
        --key "{\"#bucket\":{\"S\":\"$BUCKET\"},\"#obj\":{\"S\":\"$OBJECT\"}}" \
        --update-expression "SET #ttl = :expired_ttl" \
        --expression-attribute-names '{"#bucket":"Bucket","#obj":"Object","#ttl":"TTL"}' \
        --expression-attribute-values "{\":expired_ttl\":{\"N\":\"$EXPIRED_TTL\"}}" \
        --return-values UPDATED_NEW
fi

echo "TTL expiration completed. Item should be processed by TTL within minutes but max 48 hours."
