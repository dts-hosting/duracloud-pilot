# DuraCloud (pilot)

DuraCloud is a set of serverless components built using AWS services centered
around digital preservation use cases. It provides configuration and features
on top of AWS S3 to support long term access to and preservation of files.

## Documentation

- [User Documentation](#)
- [Technical Documentation](TECHNICAL.md)
- [Developer Documentation](DEV.md)

## Prerequisites

- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [Docker](https://docs.docker.com/engine/install/)
- [Go](https://go.dev/doc/install)
- [Saw](https://github.com/TylerBrock/saw) (for viewing logs)
- [Terraform](https://developer.hashicorp.com/terraform)

## Quick Start (development only)

1. Configure AWS credentials with a profile.

If performing these steps on behalf of Lyrasis, request information for the
"duracloudexp" experimental account.

2. Create a `.env` file with:

```
AWS_ACCOUNT_ID=your-account-id
AWS_PROFILE=your-profile-name
AWS_REGION=your-region
ALERT_EMAIL=your-email-address
PROJECT_NAME=your-project-name
STACK_NAME=your-stack-name
```

- `AWS_ACCOUNT_ID`: aws account id for a profile in your aws config
- `AWS_PROFILE`: match a profile name from your aws config (or `default`)
- `AWS_REGION`: match the region set for your aws profile
- `ALERT_EMAIL`: email address for alerts (omit or use "" to disable)
- `PROJECT_NAME`: used to create resources needed by Terraform and build tasks
- `STACK_NAME`: choose a unique prefix to apply to AWS resources created by Terraform

3. Bootstrap a project:

```bash
make bootstrap
```

This creates an S3 bucket and an ECR repository per function using the project name:

- `https://your-project-name.s3.amazonaws.com`
- `${ACCOUNT_ID}$.dkr.ecr.${REGION}$.amazonaws.com/your-project-name/${function}`

Bootstrapping only needs to performed once per AWS account. Additional projects
and stacks do not need to be bootstrapped.

4. Create Terraform backend config:

```bash
make backend-config
```

This creates `duracloud.tfbackend` using your `.env` values.

Then:

```bash
make terraform-init
```

If everything is prepared correctly the Terraform command should succeed. Resolve
any issues before proceeding.

5. Build and deploy:

```bash
make docker-pull # pulls required Docker base images
make docker-build # builds all DuraCloud function Docker images
make docker-push # pushes all DuraCloud function Docker images to ECR
```

The first build and push may take a few minutes to complete.

```bash
# deploy
make terraform-plan # review output before deployment
make terraform-apply # deploy the Terraform plan to create AWS resources
```

The apply step will take ~1-2 mins the first time it is run.

After initial deployment if you only want to build and push a single
function (i.e. a typical development workflow) then you can use:

```bash
make docker-deploy-function function=${function}
# for example:
make docker-deploy-function function=bucket-requested
```

This updates the image in ECR so subsequent invocations of the function
will use the new image, and it should be a quick process.

6. Get test user credentials:

```bash
make test-user-credentials
```

7. To clean up:

```bash
make workflow-cleanup # this empties s3 and table resources
make terraform-destroy # deletes all AWS resources
```

## Reset

If multiple people work on the same stack using different architectures
(i.e. Mac vs. Linux) then Terraform will need to be updated each time
and the functions re-built, re-pushed and re-deployed:

```bash
# set stack in .env
make terraform-plan
make terraform-apply
make docker-redeploy
```

So ideally this repository is only directly used for stacks owned
by a single developer unless collaborating using the same PC type
(arm vs x86).

## Summary

> Setting `STACK_NAME` uniquely allows for multiple deployments to the same account. Created resources are prefixed with the `STACK_NAME`. Whenever you run a Terraform task the backend state key is updated to use the stack name from `.env` allowing for multiple stacks to be deployed. To collaborate on a stack use the same project name and stack name as another developer.

## Common Tasks

### Testing Workflows

```bash
# Create buckets
make workflow-upload \
  file=files/create-buckets.txt bucket=your-stack-name-bucket-requested
make output-logs func=bucket-requested interval=5m

# Upload a file (adds record to checksum and scheduler tables)
make workflow-upload \
  file=files/upload-me.txt bucket=your-stack-name-private
make output-logs func=file-uploaded interval=5m
# the file was uploaded to: your-stack-name-private/upload-me.txt

# Trigger checksum verification by expiring ttl
make workflow-expire-ttl \
  file=upload-me.txt bucket=your-stack-name-private
make output-logs func=checksum-verification interval=5m

# Force a checksum failure
make workflow-checksum-fail \
  file=upload-me.txt bucket=your-stack-name-private
make output-logs func=checksum-failure interval=5m

# Delete a file (removes record from checksum and scheduler tables)
make workflow-delete \
  file=upload-me.txt bucket=your-stack-name-private
make output-logs func=file-deleted interval=5m

# Generate a checksum csv report (uploads to managed bucket: exports)
# We are using a prefab export files for relatively immediate results:
# files/manifest-files.json,files/abcdef123456.json.gz,files/abcdef654321.json.gz
make workflow-checksum-report
make output-logs func=checksum-export-csv-report interval=5m

# Generate a storage html report (uploads to managed bucket: reports)
# This is not useful for bucket level storage stats given cloudwatch metrics
# delay, but is fine for top level prefix counts.
# The bucket param is used as the destination for some test/tmp files
make workflow-storage-report bucket=your-stack-name-private
make output-logs func=report-generator interval=5m
```

### Viewing Logs

```bash
make output-logs func=checksum-export-csv-report interval=5m
make output-logs func=checksum-verification interval=5m
```

### Managing Buckets

```bash
make bucket-manager action=list
make bucket-manager action=create bucket=your-stack-name-private
make bucket-manager action=empty bucket=your-stack-name-private
make bucket-manager action=delete bucket=your-stack-name-private
```

### Running Functions

Ensure the functions have been deployed. There is no run local option.

```bash
make run-function \
  function=checksum-exporter \
  event=events/checksum-exporter/event.json

# Note: this supposes the file in the event payload exists
make run-function \
  function=checksum-export-csv-report \
  event=events/checksum-export-csv-report/event.json

make run-function \
  function=report-generator \
  event=events/no-event/event.json
```

### Running Tests

```bash
# run local tests only
go test ./internal/... -v

# run all tests, including those that require AWS integration
make test
```

### Processing DLQ manually

```bash
export AWS_PROFILE=profile
aws stepfunctions start-execution \
  --state-machine-arn arn:aws:states:${REGION}:${ACCOUNT}:stateMachine:${STACK}-dlq-redrive \
  --output json

# Check in on it
sleep 10
aws stepfunctions get-execution-history \
    --execution-arn arn:aws:states:${REGION}:${ACCOUNT}:execution:${STACK}-dlq-redrive:${ARN} \
    --output json | jq '.events[] | select(.type == "TaskSucceeded" or .type == "TaskFailed") | {type: .type, taskName: .previousEventId, stateEnteredEventId}'
```

---

For system architecture and component details see the [Technical Documentation](TECHNICAL.md).
