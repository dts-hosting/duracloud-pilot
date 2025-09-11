#!/bin/bash

set +e

make docker-tag-function function=bucket-requested
make docker-tag-function function=checksum-export-csv-report
make docker-tag-function function=checksum-exporter
make docker-tag-function function=checksum-failure
make docker-tag-function function=checksum-verification
make docker-tag-function function=file-deleted
make docker-tag-function function=file-uploaded
make docker-tag-function function=report-generator

docker push public.ecr.aws/d8h1c9w2/duracloud/bucket-requested:latest
docker push public.ecr.aws/d8h1c9w2/duracloud/checksum-export-csv-report:latest
docker push public.ecr.aws/d8h1c9w2/duracloud/checksum-exporter:latest
docker push public.ecr.aws/d8h1c9w2/duracloud/checksum-failure:latest
docker push public.ecr.aws/d8h1c9w2/duracloud/checksum-verification:latest
docker push public.ecr.aws/d8h1c9w2/duracloud/file-deleted:latest
docker push public.ecr.aws/d8h1c9w2/duracloud/file-uploaded:latest
docker push public.ecr.aws/d8h1c9w2/duracloud/report-generator:latest
