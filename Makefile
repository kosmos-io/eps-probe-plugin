GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= '$(shell hack/version.sh)'

# Images management
REGISTRY?="ghcr.io/kosmos-io"
REGISTRY_USER_NAME?=""
REGISTRY_PASSWORD?=""
REGISTRY_SERVER_ADDRESS?=""

TARGETS :=  eps-probe-plugin  \

# Build code.

.PHONY: $(TARGETS)
$(TARGETS):
	@echo "build binaries"

# Build image.
IMAGE_TARGET=$(addprefix image-, $(TARGETS))
.PHONY: $(IMAGE_TARGET)
$(IMAGE_TARGET):
	@echo "build images"

images: $(IMAGE_TARGET)

# Build and push multi-platform image to DockerHub
# todo
upload-images: images
	@echo "push images to $(REGISTRY)"

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