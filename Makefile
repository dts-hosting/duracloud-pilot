SHELL:=/bin/bash

.PHONY: build
build: ## Build the project (images, artifacts, etc.)
	@sam build && sam validate

.PHONY: delete
delete: ## Delete a deployed stack
	@sam delete --stack-name $(stack)

.PHONY: deploy
deploy: ## Deploy stack to AWS
	@sam deploy --stack-name $(stack)

.PHONY: invoke
invoke: ## Invoke a function using SAM CLI
	@sam local invoke $(func) --event $(event)

.PHONY: pull
pull: ## Pull required docker images
	@docker pull public.ecr.aws/docker/library/golang:1.24
	@docker pull public.ecr.aws/lambda/provided:al2023

.PHONY: test
test: ## Run all tests
	@go test -v ./...

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
