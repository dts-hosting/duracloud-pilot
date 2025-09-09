# DuraCloud (pilot)

DuraCloud is a serverless application built using AWS services that provides file storage management with built-in data integrity verification through checksums.

## Documentation

- [User Documentation](#)
- [Technical Documentation](technical-documentation.md) - Comprehensive overview of the system architecture, components, workflows, and security model

## Prerequisites

- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [Docker](https://docs.docker.com/engine/install/)
- [Go](https://go.dev/doc/install)
- [Terraform](https://developer.hashicorp.com/terraform)

## Quick Start (development only)

1. Configure AWS credentials (via profile or environment variables).

2. Create a `.env` file with:

```
AWS_ACCOUNT_ID=your-account-id
AWS_PROFILE=your-profile-name
AWS_REGION=your-region
PROJECT_NAME=your-project-name
STACK_NAME=your-stack-name
```

- `AWS_ACCOUNT_ID` is the aws account id for a profile in your aws config
- `AWS_PROFILE` should match a profile name from your aws config
- `AWS_REGION` is the region you want to use with your aws profile
- `PROJECT_NAME` is used to create resources needed by Terraform and build tasks
- `STACK_NAME` choose a unique prefix to apply to AWS resources created by Terraform

_Note: for Lyrasis testing use `duracloud-pilot` as the project name._

3. Bootstrap a project _(one time only, skip this if resources already exist):_

```bash
make bootstrap
```

This creates an S3 bucket and an ECR repository per function

- `https://your-project-name.s3.amazonaws.com`
- `${ACCOUNT_ID}$.dkr.ecr.${REGION}$.amazonaws.com/your-project-name/${function}`

_Note: skip this step for Lyrasis testing as the resources already exist._

4. Create Terraform backend config:

```bash
cp duracloud.tfbackend.EXAMPLE duracloud.tfbackend
```

Update the `duracloud.tfbackend` config to reference your actual project
and stack name.

_Note: we cannot use environment variables in `tfbackend` so need to re-specify
the project and stack name here._

_Take care to not reference another users stack name for your `key`!_

Then:

```bash
make terraform-init
```

If everything is prepared correctly the Terraform command should succeed. Resolve
any issues before proceeding.

5. Build and deploy:

```bash
# build
make docker-pull # pulls required Docker base images
make docker-build # builds all DuraCloud function Docker images
make docker-push # pushes all DuraCloud function Docker images to ECR
```

The first build and push may take a few minutes to complete.

```bash
# deploy
make terraform-plan # review output before deployment
make terraform-apply # deploy the Terraform plan
```

After initial deployment if you only want to build and push a single
function (i.e. a typical development workflow) then you can:

```bash
make docker-deploy function=${function}
```

This updates the image in ECR so subsequent invocations of the function
will use the new image, and it should be a quick process.

6. Get test user credentials:

```bash
make test-user-credentials
# make creds stack=your-stack-name
```

7. To clean up:

```bash
make workflow-cleanup
# make cleanup stack=your-stack-name

make terraform-destroy
# make delete stack=your-stack-name
```

## Summary

> **Note**: Setting `stack` uniquely allows for multiple deployments to the same account. Created resources are prefixed with the `stack` name.

For detailed build and configuration instructions, see the [Developer Guidelines](guidelines.md).

## Common Tasks

### Testing Workflows

```bash
# Create buckets
make file-copy file=files/create-buckets.txt bucket=your-stack-name-bucket-requested

# Upload a file (adds record to checksum and scheduler tables)
make file-copy file=files/upload-me.txt bucket=your-stack-name-pilot-ex-testing123

# Trigger checksum verification (file must exist in bucket)
make expire-ttl stack=your-stack-name file=upload-me.txt bucket=your-stack-name-pilot-ex-testing123

# Force a checksum failure (file must exist in bucket)
make checksum-fail stack=your-stack-name file=upload-me.txt bucket=your-stack-name-pilot-ex-testing123
make logs func=checksum-failure stack=your-stack-name interval=5m

# Delete a file (removes record from checksum and scheduler tables) (file must exist in bucket)
make file-delete file=upload-me.txt bucket=your-stack-name-pilot-ex-testing123 # confirm triggered

# Generate a checksum csv report (uploads to managed bucket under fixed key)
make report-csv file=files/abcdef123456.json.gz stack=your-stack-name
```

### Viewing Logs

```bash
make logs func=checksum-export-csv-report stack=your-stack-name interval=5m
make logs func=checksum-verification stack=your-stack-name interval=5m
```

### Managing Buckets

```bash
make bucket action=list
make bucket action=create bucket=your-stack-name-tmp
make bucket action=empty bucket=your-stack-name-tmp
make bucket action=delete bucket=your-stack-name-tmp
```

### Running Functions

Locally (note running locally is for basic debugging purposes only and may require additional configuration):

```bash
cp events/checksum-export-csv-report/event.json event.json
# update event.json to an appropriate bucket (i.e. your-stack-name)
make invoke func=ChecksumExportCSVReportFunction event=event.json

make invoke func=FileUploadedFunction event=events/file-uploaded/event.json
```

Remotely:

```bash
make invoke-remove func=ChecksumExportCSVReportFunction event=events/checksum-export-csv-report/event.json stack=your-stack-name
make invoke-remote func=ChecksumExporterFunction event=events/checksum-exporter/event.json stack=your-stack-name
make invoke-remote func=ReportGeneratorFunction event=events/no-event/event.json stack=your-stack-name
```

### Running Tests

```bash
make test stack=your-stack-name
```

For detailed information about testing, debugging, and development practices, see the [Developer Guidelines](guidelines.md).

For comprehensive system architecture and component details, see the [Technical Documentation](technical-documentation.md).
