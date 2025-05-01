# DuraCloud (pilot)

Requires:

- [AWS cli](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [AWS sam](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- [Docker](https://docs.docker.com/engine/install/)
- [Go](https://go.dev/doc/install)

## Usage

```bash
make pull # download the required docker images
make build # prebuild images

# local testing
make invoke func=BucketCreatedFunction event=events/bucket-created/event.json

# deploy
AWS_PROFILE=duracloudexp make deploy stack=duracloud-lyrasis

# destroy
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh duracloud-lyrasis-event-logs empty
AWS_PROFILE=duracloudexp make delete stack=duracloud-lyrasis
```

- Setting `stack` uniquely allows for multiple deployments to the same account.
- Created resources are prefixed with the `stack` name.

## Utility scripts

The `bucket-manager` script can be used to create, clear and delete buckets:

```bash
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 create
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 empty
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 delete
```
