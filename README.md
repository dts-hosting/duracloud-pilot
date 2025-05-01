# DuraCloud (pilot)

Requires:

- [AWS cli](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [AWS sam](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- [Docker](https://docs.docker.com/engine/install/)
- [Go](https://go.dev/doc/install)

## Usage

```bash
make pull # download the required docker images
```

## Utility scripts

```bash
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 create
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 empty
AWS_PROFILE=duracloudexp ./scripts/bucket-manager.sh  duracloud-pilot-bucket1 delete
```
