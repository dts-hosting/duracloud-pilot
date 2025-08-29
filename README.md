# DuraCloud (pilot)

DuraCloud is a serverless application built using AWS SAM that provides robust file storage management with built-in data integrity verification through checksums.

## Documentation

- [Technical Documentation](technical-documentation.md) - Comprehensive overview of the system architecture, components, workflows, and security model
- [Developer Guidelines](guidelines.md) - Detailed information for developers working on the project

## Prerequisites

- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [AWS SAM](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- [Docker](https://docs.docker.com/engine/install/)
- [Go](https://go.dev/doc/install)
- [Node.js](https://nodejs.org/en) (for documentation site)

## Quick Start

1. Configure AWS credentials (via profile or environment variables)

2. Create a `.env` file with your AWS profile:

```
STACK_NAME=your-profile-name
```

3. Build and deploy:

```bash
make pull
make build
make deploy-only stack=your-stack-name
# you can run the build and deploy tasks in one step using:
make deploy stack=your-stack-name
```

4. Get test user credentials:

```bash
make creds stack=your-stack-name
```

5. To clean up:

```bash
make cleanup stack=your-stack-name
make delete stack=your-stack-name
```

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

Locally:

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
```

### Running Tests

```bash
make test stack=your-stack-name
```

For detailed information about testing, debugging, and development practices, see the [Developer Guidelines](guidelines.md).

For comprehensive system architecture and component details, see the [Technical Documentation](technical-documentation.md).
