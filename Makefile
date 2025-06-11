SHELL:=/bin/bash

ARCH:=$(shell uname -m)

ifeq ($(ARCH),x86_64)
    LAMBDA_ARCH:=x86_64
else ifeq ($(ARCH),arm64)
    LAMBDA_ARCH:=arm64
else
    $(error Unsupported architecture $(ARCH))
endif

.PHONY: bucket
bucket: ## Run a bucket manager script command
	@./scripts/bucket-manager.sh $(action) $(bucket)

.PHONY: build
build: ## Build the project (images, artifacts, etc.)
	@sam build --parameter-overrides LambdaArchitecture=$(LAMBDA_ARCH) && sam validate

.PHONY: creds
creds: ## Output the test user access key and secret
	@aws ssm get-parameter --name "/$(stack)/iam/test/access-key-id"
	@aws ssm get-parameter --name "/$(stack)/iam/test/secret-access-key"

.PHONY: delete
delete: ## Delete a deployed stack
	@sam delete --stack-name $(stack)

.PHONY: deploy
deploy: build ## Deploy stack to AWS
	@sam deploy --stack-name $(stack) --parameter-overrides LambdaArchitecture=$(LAMBDA_ARCH)

.PHONY: docs-build
docs-build: ## Build the docs site
	@cd docs-src && npm run build

.PHONY: docs-dev
docs-dev: ## Start the docs dev server
	@cd docs-src && npm run dev

.PHONY: docs-install
docs-install: ## Install docs dependencies
	@npm install && cd docs-src && npm install

.PHONY: invoke
invoke: ## Invoke a function using SAM CLI locally
	@sam local invoke $(func) --event $(event) --parameter-overrides LambdaArchitecture=$(LAMBDA_ARCH)

.PHONY: lint
lint: ## Run linters
	@docker run -t --rm -v .:/app -w /app golangci/golangci-lint:latest golangci-lint run || true
	@gofmt -w .
	@npm run format

.PHONY: logs
logs: ## Output logs to console
	@./scripts/output-logs.sh $(func) $(stack) $(interval)

.PHONY: pull
pull: ## Pull required docker images
	@docker pull public.ecr.aws/docker/library/golang:1.24
	@docker pull public.ecr.aws/lambda/provided:al2023

.PHONY: test
test: ## Run all tests
	@STACK_NAME=$(stack) go test -v ./...
	@./scripts/bucket-manager.sh empty $(stack)-bucket-requested > /dev/null

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
