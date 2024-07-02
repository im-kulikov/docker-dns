-include .env

SHELL := /bin/sh

IMAGE_NAME=ghcr.io/im-kulikov/docker-dns


.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(firstword $(MAKEFILE_LIST)) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: PLATFORMS=linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6
build: ## Build and push docker image
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE_NAME) --push .