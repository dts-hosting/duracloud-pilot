# DuraCloud (pilot)

Requires:

- [AWS cli](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [AWS sam](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- [Docker](https://docs.docker.com/engine/install/)
- [Go](https://go.dev/doc/install)

To view logs install `saw` (using Go is recommended):

```bash
go install github.com/TylerBrock/saw@latest
```

The docs site requires [Node.js](https://nodejs.org/en). Install using `nvm`:

- [nvm](https://github.com/nvm-sh/nvm)

```bash
nvm install
nvm use
make docs-install
```

## Usage

```bash
make pull # download the required docker images
make build # prebuild images

# deploy
AWS_PROFILE=duracloudexp make deploy stack=duracloud-lyrasis

# get the support user access key and secret
AWS_PROFILE=duracloudexp make creds stack=duracloud-lyrasis

# trigger: test bucket creation
AWS_PROFILE=duracloudexp aws s3 cp files/create-buckets.txt s3://duracloud-lyrasis-bucket-requested/

# output logs (optional: interval=30m, default is 5m)
AWS_PROFILE=duracloudexp make logs func=BucketRequestedFunction stack=duracloud-lyrasis

# destroy
AWS_PROFILE=duracloudexp make bucket action=empty bucket=duracloud-lyrasis-bucket-requested
AWS_PROFILE=duracloudexp make delete stack=duracloud-lyrasis
```

- Setting `stack` uniquely allows for multiple deployments to the same account.
- Created resources are prefixed with the `stack` name.

## Utility tasks

The `make bucket` task can be used to create, clear and delete buckets:

```bash
AWS_PROFILE=duracloudexp make bucket action=list
AWS_PROFILE=duracloudexp make bucket action=create bucket=duracloud-pilot-bucket1
AWS_PROFILE=duracloudexp make bucket action=empty bucket=duracloud-pilot-bucket1
AWS_PROFILE=duracloudexp make bucket action=delete bucket=duracloud-pilot-bucket1
```

The `make invoke` task can be used to run functions locally:

```bash
make invoke func=FileUploadedFunction event=events/file-uploaded/event.json
make invoke func=FileDeletedFunction event=events/file-deleted/event.json
make invoke func=ChecksumVerificationFunction event=events/checksum-verification/event.json
make invoke func=ChecksumFailureFunction event=events/checksum-failure/event.json
make invoke func=ChecksumExporterFunction event=events/checksum-exporter/event.json
```

However, results will vary depending on how strongly the function depends on
deployed resources. In most cases this is only useful for debugging the initial
configuration and event payloads.

## Tests

The stack indicated by `STACK_NAME` must be deployed first.

```bash
AWS_PROFILE=duracloudexp make test stack=duracloud-lyrasis
```
