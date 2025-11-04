#!/bin/bash
# ./scripts/bootstrap.sh duracloud-pilot - create s3 bucket and ecr repository

NAME=${1}
REGION=${2:-us-west-2}

aws s3api create-bucket \
  --region ${REGION} \
  --create-bucket-configuration LocationConstraint="${REGION}" \
  --acl private \
  --bucket "${NAME}"

aws s3api put-bucket-versioning \
  --bucket "${NAME}" \
  --versioning-configuration Status=Enabled

aws s3api put-public-access-block \
  --bucket "${NAME}" \
  --public-access-block-configuration BlockPublicAcls=True,IgnorePublicAcls=True,BlockPublicPolicy=True,RestrictPublicBuckets=True

# Create ECR repositories for each function
FUNCTIONS=(
  "bucket-requested"
  "checksum-export-csv-report"
  "checksum-exporter"
  "checksum-failure"
  "checksum-verification"
  "file-deleted"
  "file-uploaded"
  "inventory-unwrap"
  "report-generator"
)

for function in "${FUNCTIONS[@]}"; do
  echo "Creating ECR repository for ${NAME}/${function}..."
  aws ecr create-repository \
    --repository-name "${NAME}/${function}" || true

  echo "Setting lifecycle policy for ${NAME}/${function}..."
  aws ecr put-lifecycle-policy \
    --repository-name "${NAME}/${function}" \
    --lifecycle-policy-text file://files/ecr-lifecycle-policy.json || true
done
