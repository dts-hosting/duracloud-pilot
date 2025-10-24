SHELL:=/bin/bash

-include .env
export $(shell sed 's/=.*//' .env 2>/dev/null)

ARCH:=$(shell uname -m)

ifeq ($(ARCH),x86_64)
    LAMBDA_ARCH:=x86_64
else ifeq ($(ARCH),arm64)
    LAMBDA_ARCH:=arm64
else
    $(error Unsupported architecture $(ARCH))
endif

TF_VARS := TF_VAR_arch=$(LAMBDA_ARCH) \
	TF_VAR_repo=$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT_NAME) \
	TF_VAR_stack=$(STACK_NAME)

.PHONY: backend-config
backend-config: ## Generate duracloud.tfbackend from template
	@cp duracloud.tfbackend.EXAMPLE duracloud.tfbackend
	@if [ "$$(uname)" = "Darwin" ]; then \
		sed -i '' 's/your-project-name/$(PROJECT_NAME)/g' duracloud.tfbackend; \
		sed -i '' 's/your-stack-name.tfstate/$(STACK_NAME).tfstate/g' duracloud.tfbackend; \
		sed -i '' 's/your-region/$(AWS_REGION)/g' duracloud.tfbackend; \
	else \
		sed -i 's/your-project-name/$(PROJECT_NAME)/g' duracloud.tfbackend; \
		sed -i 's/your-stack-name.tfstate/$(STACK_NAME).tfstate/g' duracloud.tfbackend; \
		sed -i 's/your-region/$(AWS_REGION)/g' duracloud.tfbackend; \
	fi

.PHONY: bootstrap
bootstrap: ## Create project S3 bucket and ECR repository resources
	@./scripts/bootstrap.sh $(PROJECT_NAME)

.PHONY: bucket-manager
bucket-manager: ## Run a bucket manager script command
	@./scripts/bucket-manager.sh $(action) $(bucket)

.PHONY: docker-build
docker-build: ## Build the docker images for all functions
docker-build:
	@$(MAKE) docker-build-function function=bucket-requested
	@$(MAKE) docker-build-function function=checksum-export-csv-report
	@$(MAKE) docker-build-function function=checksum-exporter
	@$(MAKE) docker-build-function function=checksum-failure
	@$(MAKE) docker-build-function function=checksum-verification
	@$(MAKE) docker-build-function function=file-deleted
	@$(MAKE) docker-build-function function=file-uploaded
	@$(MAKE) docker-build-function function=report-generator

.PHONY: docker-build-function
docker-build-function: ## Build a specific function
	@docker build . --build-arg FUNCTION_NAME=$(function) \
		-t $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT_NAME)/$(function):$(STACK_NAME)

.PHONY: docker-deploy-function
docker-deploy-function: ## Build, push and update a specific function
	@$(MAKE) docker-build-function function=$(function)
	@$(MAKE) docker-push-function function=$(function)
	@$(MAKE) update-function function=$(function)

.PHONY: docker-redeploy
docker-redeploy: ## Build, push and redeploy all functions
	@$(MAKE) docker-deploy-function function=bucket-requested
	@$(MAKE) docker-deploy-function function=checksum-export-csv-report
	@$(MAKE) docker-deploy-function function=checksum-exporter
	@$(MAKE) docker-deploy-function function=checksum-failure
	@$(MAKE) docker-deploy-function function=checksum-verification
	@$(MAKE) docker-deploy-function function=file-deleted
	@$(MAKE) docker-deploy-function function=file-uploaded
	@$(MAKE) docker-deploy-function function=report-generator

.PHONY: docker-pull
docker-pull: ## Pull required docker images
	@docker pull public.ecr.aws/docker/library/golang:1.24
	@docker pull public.ecr.aws/lambda/provided:al2023

.PHONY: docker-push
docker-push: ## Push the docker images for all functions
docker-push:
	@$(MAKE) docker-push-function function=bucket-requested
	@$(MAKE) docker-push-function function=checksum-export-csv-report
	@$(MAKE) docker-push-function function=checksum-exporter
	@$(MAKE) docker-push-function function=checksum-failure
	@$(MAKE) docker-push-function function=checksum-verification
	@$(MAKE) docker-push-function function=file-deleted
	@$(MAKE) docker-push-function function=file-uploaded
	@$(MAKE) docker-push-function function=report-generator

.PHONY: docker-push-function
docker-push-function: ## Push a specific function
	@docker push \
		$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT_NAME)/$(function):$(STACK_NAME)

.PHONY: lint
lint: ## Run linters
	@docker run -t --rm -v .:/app -w /app golangci/golangci-lint:latest golangci-lint run || true
	@gofmt -w .
	@terraform fmt .

.PHONY: output-logs
output-logs: ## Output logs to console
	@./scripts/output-logs.sh $(func) $(STACK_NAME) $(interval)

.PHONY: run-function
run-function: ## Invoke a deployed function
	@aws lambda invoke \
		--function-name $(STACK_NAME)-$(function) \
		--payload file://$(event) \
		--cli-binary-format raw-in-base64-out \
		--output text \
		/dev/stdout

.PHONY: terraform-apply
terraform-apply: backend-config terraform-init ## Run Terraform apply
	@$(TF_VARS) terraform apply -auto-approve

.PHONY: terraform-destroy
terraform-destroy: backend-config terraform-init ## Run Terraform destroy
	@$(TF_VARS) terraform destroy

.PHONY: terraform-init
terraform-init: backend-config ## Run Terraform init
	@terraform init -backend-config="duracloud.tfbackend" -reconfigure -upgrade

.PHONY: terraform-plan
terraform-plan: backend-config terraform-init ## Run Terraform plan
	@$(TF_VARS) terraform plan

.PHONY: test
test: ## Run all tests and cleanup resources
	@go test -count 1 -v ./...
	@printf "\n\n\nTests ran successfully cleaning up ...\n\n\n"
	@$(MAKE) workflow-cleanup

.PHONY: test-user-credentials
test-user-credentials: ## Output the test user access key and secret
	@aws ssm get-parameter --name "/$(STACK_NAME)/iam/test/access-key-id"
	@aws ssm get-parameter --name "/$(STACK_NAME)/iam/test/secret-access-key"

.PHONY: update-function
update-function: ## Update the function code using latest Docker img
	@aws lambda update-function-code \
		--function-name $(STACK_NAME)-$(function) \
		--image-uri $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT_NAME)/$(function):$(STACK_NAME)

.PHONY: update-functions
update-functions: ## Update all functions using latest Docker img
	@$(MAKE) update-function function=bucket-requested
	@$(MAKE) update-function function=checksum-export-csv-report
	@$(MAKE) update-function function=checksum-exporter
	@$(MAKE) update-function function=checksum-failure
	@$(MAKE) update-function function=checksum-verification
	@$(MAKE) update-function function=file-deleted
	@$(MAKE) update-function function=file-uploaded
	@$(MAKE) update-function function=report-generator

.PHONY: workflow-checksum-fail
workflow-checksum-fail: ## Force a checksum failure (bucket=name file=key)
	@./scripts/checksum-fail.sh $(STACK_NAME) $(bucket) $(file)

.PHONY: workflow-checksum-report
workflow-checksum-report: ## Generate a checksum csv report
	@aws s3 cp files/abcdef123456.json.gz \
		s3://$(STACK_NAME)-managed/exports/checksum-table/2025-08-25/AWSDynamoDB/01234567890123456789/data/abcdef123456.json.gz

.PHONY: workflow-cleanup
workflow-cleanup: ## Cleanup (clear out) deployed bucket and table resources
	@./scripts/cleanup-stack.sh $(STACK_NAME)

.PHONY: workflow-delete
workflow-delete: ## Delete a file from a bucket (root level only)
	@aws s3 rm s3://$(bucket)/$(file)

.PHONY: workflow-expire-ttl
workflow-expire-ttl: ## Expire TTL for checksum verification (bucket=name file=key)
	@./scripts/expire-ttl.sh $(STACK_NAME) $(bucket) $(file)

.PHONY: workflow-storage-report
workflow-storage-report: ## Generate a storage html report
	@aws s3 cp ./files/upload-me.txt s3://$(bucket)/files/file1.txt
	@aws s3 cp ./files/upload-me.txt s3://$(bucket)/files/file2.txt
	@aws s3 cp ./files/upload-me.txt s3://$(bucket)/files/file3.txt
	@$(MAKE) run-function \
		function=report-generator event=events/no-event/event.json

.PHONY: workflow-upload
workflow-upload: ## Copy a file to a bucket (root level only)
	@aws s3 cp $(file) s3://$(bucket)/

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-45s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
