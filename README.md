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

Export a profile or otherwise configure AWS credentials. One option is to
create a `.env` file and add AWS environment variables to it:

```txt
AWS_PROFILE=default
```

That's all the setup needed for using the provided `make` tasks.

```bash
make pull # download the required docker images
make build # prebuild images

# deploy
make deploy stack=duracloud-lyrasis

# get the support user access key and secret
make creds stack=duracloud-lyrasis

# destroy
make cleanup stack=duracloud-lyrasis
make delete stack=duracloud-lyrasis
```

- Setting `stack` uniquely allows for multiple deployments to the same account.
- Created resources are prefixed with the `stack` name.

## Workflow testing

```bash
# trigger: bucket creation function
make file-copy file=files/create-buckets.txt bucket=duracloud-lyrasis-bucket-requested
make bucket action=list # list buckets, should shortly have the newly created ones

# trigger: file uploaded function
make file-copy file=files/upload-me.txt bucket=duracloud-lyrasis-pilot-ex-testing123

# trigger: checksum verification
make expire-ttl stack=duracloud-lyrasis file=upload-me.txt bucket=duracloud-lyrasis-pilot-ex-testing123

# trigger: checksum failure notification
# TODO: set checksum status false (message = "forcibly induced error") for an item

# trigger: file deleted function
make file-delete file=upload-me.txt bucket=duracloud-lyrasis-pilot-ex-testing123
```

To view logs:

```bash
# output logs (optional: interval=30m, default is 5m)
make logs func=BucketRequestedFunction stack=duracloud-lyrasis
```

## Utility tasks

The `make bucket` task can be used to create, clear and delete buckets:

```bash
make bucket action=list
make bucket action=create bucket=duracloud-lyrasis-tmp
make bucket action=empty bucket=duracloud-lyrasis-tmp
make bucket action=delete bucket=duracloud-lyrasis-tmp
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

The `make invoke-remote` task can be used to run deployed functions:

```bash
make invoke-remote func=ChecksumExporterFunction event=events/checksum-exporter/event.json stack=duracloud-lyrasis
```

To see the status of an export:

```bash
AWS_PROFILE=$PROFILE aws dynamodb describe-export --export-arn $ARN
```

## Tests

The stack indicated by `STACK_NAME` must be deployed first.

```bash
make test stack=duracloud-lyrasis
```
