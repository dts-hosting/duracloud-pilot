#!/bin/bash

STACK=${1:-duracloud-lyrasis}
BUCKET=${2}
OBJECT=${3}

TABLE_NAME="${STACK}-checksum-table"

if [ -n "$BUCKET" ] && [ -n "$OBJECT" ]; then
    echo "Forcing checksum failure for: $BUCKET/$OBJECT"
    aws dynamodb update-item \
        --table-name "$TABLE_NAME" \
        --key "{
            \"BucketName\": {\"S\": \"$BUCKET\"},
            \"ObjectKey\": {\"S\": \"$OBJECT\"}
        }" \
        --update-expression "SET LastChecksumSuccess = :false_val" \
        --expression-attribute-values "{
            \":false_val\": {\"BOOL\": false}
        }" \
        --return-values UPDATED_NEW
else
    echo "Usage: $0 [stack-name] <bucket-name> <object-key>"
    echo "Example: $0 duracloud-lyrasis my-bucket path/to/file.txt"
    exit 1
fi

echo "LastChecksumSuccess field updated to false."
echo "This will trigger the checksum failure handler on the next DynamoDB stream event."