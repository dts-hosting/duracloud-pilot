SHELL:=/bin/bash

.PHONY: pull
pull: ## Pull required docker images
	@docker pull public.ecr.aws/docker/library/golang:1.24
	@docker pull public.ecr.aws/lambda/provided:al2023

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
