# Development

Creating a new function.

## Code

Function handlers should be added to:

- ./cmd/${function}/main.go

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
  - aws_lambda_function
  - aws_cloudwatch_log_group
- `cloudwatch.tf`
  - aws_cloudwatch_metric_alarm
- `iam.tf`
  - aws_iam_role
  - aws_iam_role_policy
  - aws_iam_role_policy_attachment
- `outputs.tf`

That's the minimum. Depending on what the function does other
resources or configuration may be required.
