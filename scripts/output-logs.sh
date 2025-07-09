#!/bin/bash
# scripts/output-logs.sh - Script to output AWS Lambda function logs using saw

# Check if function and stack name were provided
if [ -z "$1" ] || [ -z "$2" ]; then
  echo "Usage: $0 <function-name> <stack-name>"
  echo "Example: $0 checksum-verification duracloud-lyrasis"
  exit 1
fi

FUNCTION_NAME=$1
STACK_NAME=$2
INTERVAL=${3:-5m}

echo "Looking for log group for function $FUNCTION_NAME"

GROUP=$(saw groups | grep "/aws/lambda/$STACK_NAME-$FUNCTION_NAME")

if [ -z "$GROUP" ]; then
  echo "No log group found for function: $FUNCTION_NAME"
  echo "Available groups:"
  saw groups
  exit 1
fi

echo "Found log group: $GROUP"
echo "Logs:"
saw get $GROUP --start -$INTERVAL
