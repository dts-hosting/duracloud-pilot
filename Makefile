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

.PHONY: bootstrap
bootstrap: ## Create project S3 bucket and ECR repository resources
	@./scripts/bootstrap.sh $(PROJECT_NAME)

.PHONY: bucket
bucket: ## Run a bucket manager script command
	@./scripts/bucket-manager.sh $(action) $(bucket)

.PHONY: checksum-fail
checksum-fail: ## Force a checksum failure (stack=name bucket=name file=key)
	@./scripts/checksum-fail.sh $(stack) $(bucket) $(file)

.PHONY: cleanup
cleanup: ## Cleanup remote bucket and table resources for a stack
	@./scripts/cleanup-stack.sh $(stack)

.PHONY: creds
creds: ## Output the test user access key and secret
	@aws ssm get-parameter --name "/$(stack)/iam/test/access-key-id"
	@aws ssm get-parameter --name "/$(stack)/iam/test/secret-access-key"

.PHONY: delete
delete: ## Delete a deployed stack
	@sam delete --stack-name $(stack) --no-prompts

.PHONY: deploy
deploy: build ## Deploy stack to AWS
	@sam deploy --stack-name $(stack) --parameter-overrides LambdaArchitecture=$(LAMBDA_ARCH)

.PHONY: deploy-only
deploy-only: ## Deploy stack to AWS w/o running a build first
	@sam deploy --stack-name $(stack) --parameter-overrides LambdaArchitecture=$(LAMBDA_ARCH)

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

.PHONY: docker-deploy
docker-deploy: ## Build and push a specific function
	@$(MAKE) docker-build-function function=$(function)
	@$(MAKE) docker-push-function function=$(function)

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

.PHONY: docs-build
docs-build: ## Build the docs site
	@cd docs-src && npm run build

.PHONY: docs-dev
docs-dev: ## Start the docs dev server
	@cd docs-src && npm run dev

.PHONY: docs-install
docs-install: ## Install docs dependencies
	@npm install && cd docs-src && npm install

.PHONY: expire-ttl
expire-ttl: ## Expire TTL for checksum verification (bucket=name file=key)
	@./scripts/expire-ttl.sh $(stack) $(bucket) $(file)

.PHONY: file-copy
file-copy: ## Copy a file to a bucket (without prefixes)
	@aws s3 cp $(file) s3://$(bucket)/

.PHONY: file-delete
file-delete: ## Delete a file from a bucket (without prefixes)
	@aws s3 rm s3://$(bucket)/$(file)

.PHONY: invoke
invoke: ## Invoke a function using SAM CLI locally
	@sam local invoke $(func) --event $(event) --parameter-overrides LambdaArchitecture=$(LAMBDA_ARCH)

.PHONY: invoke-remote
invoke-remote: ## Invoke a deployed function remotely
	@sam remote invoke $(func) --event-file $(event) --stack-name $(stack)

.PHONY: lint
lint: ## Run linters
	@docker run -t --rm -v .:/app -w /app golangci/golangci-lint:latest golangci-lint run || true
	@gofmt -w .
	@terraform fmt .
	@npm run format

.PHONY: logs
logs: ## Output logs to console
	@./scripts/output-logs.sh $(func) $(stack) $(interval)

.PHONY: report-csv
report-csv: ## Generate a checksum csv report
	@aws s3 cp $(file) s3://$(stack)-managed/exports/checksum-table/2025-08-25/AWSDynamoDB/01234567890123456789/data/abcdef123456.json.gz

.PHONY: terraform-init
terraform-init: ## Run Terraform init
	@terraform init -backend-config="duracloud.tfbackend" -reconfigure -upgrade

.PHONY: test
test: ## Run all tests and cleanup resources
	@STACK_NAME=$(stack) go test -count 1 -v ./...
	@echo -e "\n\n\nTests ran successfully cleaning up ...\n\n\n"
	$(MAKE) cleanup stack=$(stack)

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-45s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
