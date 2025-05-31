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

# output logs (optional: interval=30m, default is 5m)
AWS_PROFILE=duracloudexp make logs func=BucketRequestedFunction stack=duracloud-lyrasis

# destroy
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh duracloud-lyrasis-bucket-requested empty
AWS_PROFILE=duracloudexp make delete stack=duracloud-lyrasis
```

- Setting `stack` uniquely allows for multiple deployments to the same account.
- Created resources are prefixed with the `stack` name.

## Testing functions

### BucketRequestedFunction

Local testing can be run using `make invoke`:

```bash
# local testing
make invoke func=BucketRequestedFunction event=events/bucket-requested/event.json
```

This can be used to test incoming event payloads.

```bash
# trigger: test bucket creation
AWS_PROFILE=duracloudexp aws s3 cp files/create-buckets.txt s3://duracloud-lyrasis-bucket-requested/
```

## Utility scripts

The `bucket-manager` script can be used to create, clear and delete buckets:

```bash
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 create
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 empty
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 delete
```
