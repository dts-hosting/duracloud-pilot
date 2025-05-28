#!/bin/bash
# scripts/bucket-manager.sh - Script to create, empty or delete buckets

BUCKET_NAME=$1
ACTION=$2

if [ -z "$BUCKET_NAME" ] || [ -z "$ACTION" ]; then
    echo "Usage: $0 <bucket-name> <action>"
    echo "Actions: create, empty, delete"
    exit 1
fi

case $ACTION in
    create)
        aws s3 mb s3://$BUCKET_NAME
    ;;
    empty)
        aws s3 rm s3://$BUCKET_NAME --recursive
    ;;
    delete)
        aws s3 rb s3://$BUCKET_NAME --force
    ;;
    *)
        echo "Invalid action. Use 'create', 'empty', or 'delete'."
        exit 1
    ;;
esac
