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

.PHONY: bucket
bucket: ## Run a bucket manager script command
	@./scripts/bucket-manager.sh $(action) $(bucket)

.PHONY: build
build: ## Build the project (images, artifacts, etc.)
	@sam build --parameter-overrides LambdaArchitecture=$(LAMBDA_ARCH) && sam validate

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

.PHONY: guidelines
guidelines: ## Copy the guidelines into LLM favored locations
	@mkdir -p .junie
	@cp guidelines.md CLAUDE.md && cp guidelines.md .junie/

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
	@npm run format

.PHONY: logs
logs: ## Output logs to console
	@./scripts/output-logs.sh $(func) $(stack) $(interval)

.PHONY: pull
pull: ## Pull required docker images
	@docker pull public.ecr.aws/docker/library/golang:1.24
	@docker pull public.ecr.aws/lambda/provided:al2023

.PHONY: test
test: ## Run all tests and cleanup resources
	@STACK_NAME=$(stack) go test -v ./...
	$(MAKE) cleanup stack=$(stack)

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help