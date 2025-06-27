#!/bin/bash

STACK=${1:-duracloud-lyrasis}

echo "Clearing buckets"

make bucket action=empty bucket=${STACK}-bucket-requested > /dev/null
make bucket action=empty bucket=${STACK}-logs > /dev/null
make bucket action=empty bucket=${STACK}-managed > /dev/null

# Clean up any dynamically created buckets
aws s3api list-buckets \
  --query "Buckets[?starts_with(Name, \`${STACK}-\`)].Name" \
  --output text | tr '\t' '\n' | \
  grep -vE '(-bucket-requested|-logs|-managed)$' | \
  xargs -I{} sh -c '
    echo "Processing bucket: $0"
    if ! make bucket action=empty bucket=$0 > /dev/null; then echo "empty failed for $0"; fi
    if ! make bucket action=delete bucket=$0 > /dev/null; then echo "delete failed for $0"; fi
  ' {}

clear_dynamodb_table() {
    local TABLE_NAME=$1
    echo "Clearing table: $TABLE_NAME"
    local temp_dir=$(mktemp -d)

    aws dynamodb scan \
        --table-name "$TABLE_NAME" \
        --projection-expression "#bucket, #obj" \
        --expression-attribute-names '{"#bucket":"Bucket","#obj":"Object"}' \
        --max-items 1000 \
        | jq -c '.Items[] | {DeleteRequest: {Key: {Bucket: .Bucket, Object: .Object}}}' \
        | split -l 25 - "$temp_dir/items_"

    # Process each batch file
    for batch_file in "$temp_dir"/items_*; do
        if [ -f "$batch_file" ]; then
            jq -s --arg tn "$TABLE_NAME" '{($tn): .}' "$batch_file" > "$temp_dir/batch.json"

            if aws dynamodb batch-write-item --request-items "file://$temp_dir/batch.json"; then
                echo "Successfully deleted batch from $TABLE_NAME"
            else
                echo "Failed to delete batch from $TABLE_NAME"
            fi
        fi
    done

    rm -rf "$temp_dir"
}

clear_dynamodb_table "${STACK}-checksum-table"
clear_dynamodb_table "${STACK}-checksum-scheduler-table"

exit 0
