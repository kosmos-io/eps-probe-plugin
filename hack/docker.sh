#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

REGISTRY=${REGISTRY:-"ghcr.io/kosmos-io"}
VERSION=${VERSION:="unknown"}
DOCKER_BUILD_ARGS=${DOCKER_BUILD_ARGS:-}

function build_images() {
  local -r target=$1
  local -r os=$2
  local -r arch=$3
  local -r platform="${os}-${arch}"
  local dockerfile="Dockerfile"

  local -r image_name="${REGISTRY}/${target}:${VERSION}"

  echo "Building image for ${platform}: ${image_name}"
  set -x
  docker build --build-arg BINARY="${target}" \
          ${DOCKER_BUILD_ARGS} \
          --tag "${image_name}" \
          --file "${REPO_ROOT}/${dockerfile}" \
          "${REPO_ROOT}/_output/bin/${platform}"
  set +x
}

build_images "$@"
