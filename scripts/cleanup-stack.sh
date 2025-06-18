#!/bin/bash

STACK=${1:-duracloud-lyrasis}

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

exit 0
