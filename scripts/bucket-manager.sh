#!/bin/bash
# scripts/bucket-manager.sh - Script to list, create, empty or delete buckets

ACTION=$1
BUCKET_NAME=$2

if [ -z "$ACTION" ]; then
    echo "Usage: $0 <action> [bucket-name]"
    echo "Actions: list, create, empty, delete"
    echo "Note: bucket-name is required for create, empty, and delete actions"
    exit 1
fi

if [ "$ACTION" != "list" ] && [ -z "$BUCKET_NAME" ]; then
    echo "Error: Bucket name is required for $ACTION action"
    echo "Usage: $0 $ACTION <bucket-name>"
    exit 1
fi

case $ACTION in
    list)
        aws s3api list-buckets
    ;;
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
        echo "Invalid action. Use 'list', 'create', 'empty', or 'delete'."
        exit 1
    ;;
esac
