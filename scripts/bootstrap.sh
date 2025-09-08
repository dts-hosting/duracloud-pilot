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

aws ecr create-repository \
  --repository-name "${NAME}"

aws ecr put-lifecycle-policy \
  --repository-name "${NAME}" \
  --lifecycle-policy-text file://files/ecr-lifecycle-policy.json
