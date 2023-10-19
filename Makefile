GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= '$(shell hack/version.sh)'

# Images management
REGISTRY?="ghcr.io/kosmos-io"
REGISTRY_USER_NAME?=""
REGISTRY_PASSWORD?=""
REGISTRY_SERVER_ADDRESS?=""

# Build code.
.PHONY: build
build:
	@make build-eps-probe-plugin GOOS=linux GOARCH=amd64
#	@make build-eps-probe-plugin GOOS=linux GOARCH=arm64
#	@make build-eps-probe-plugin GOOS=darwin GOARCH=amd64
#	@make build-eps-probe-plugin GOOS=darwin GOARCH=arm64

build-eps-probe-plugin:
	hack/build.sh eps-probe-plugin ${GOOS} ${GOARCH}

# Build image.
.PHONY: image
image: build
	@make image-eps-probe-plugin GOOS=linux GOARCH=amd64

image-eps-probe-plugin:
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) hack/docker.sh eps-probe-plugin ${GOOS} ${GOARCH}

# TODO Build and push multi-platform image to DockerHub
upload-images: image
	@echo "push images to $(REGISTRY)"
	docker tag  ${REGISTRY}/eps-probe-plugin:${VERSION}  ${REGISTRY}/eps-probe-plugin:latest
	docker push ${REGISTRY}/eps-probe-plugin:${VERSION}
	docker push ${REGISTRY}/eps-probe-plugin:latest

.PHONY: lint
lint: golangci-lint
	$(GOLANGLINT_BIN) run

golangci-lint:
ifeq (, $(shell which golangci-lint))
	GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2
GOLANGLINT_BIN=$(shell go env GOPATH)/bin/golangci-lint
else
GOLANGLINT_BIN=$(shell which golangci-lint)
endif