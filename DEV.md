# Development

Creating a new function.

## Code

Function handlers should be added to:

- `./cmd/${function}/main.go`

There should be no other files. Generally each `main.go` should
only contain:

- vars
- init
- handler
- main (calls handler)

Other code should be put in `internal` pkgs.

## Infrastructure

Resources:

- `lambda.tf`
  - aws_cloudwatch_log_group
  - aws_lambda_function
  - aws_lambda_permission
- `cloudwatch.tf`
  - aws_cloudwatch_metric_alarm
- `iam.tf`
  - aws_iam_role
  - aws_iam_role_policy
  - aws_iam_role_policy_attachment
- `variables.tf`
- `outputs.tf`

That's the ~minimum. Depending on what the function does other
resources or configuration may be required.

You will also need to update:

- `main.tf` add image uri entry
- `Makefile` update:
  - `docker-build`, `docker-push`, `docker-redeploy` and `update-functions`
- `docker-build-push.yml` update the matrix
- `scripts/bootstrap.sh` add to function list (requires bootstrap to be run)

TODO: refactor parts to read function names from cmd dir.

## Testing

```bash
make bootstrap # if ecr repo needs to be created
make docker-build-function function=$function
make docker-push-function function=$function
make terraform-apply

# iterating
make docker-deploy-function function=$function
```
