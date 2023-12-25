#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

GO_PACKAGE="github.com/kosmos.io/eps-probe-plugin"

function util::get_version() {
  git describe --tags --dirty --always
}

function util::version_ldflags() {
  # Git information
  GIT_VERSION=$(util::get_version)
  GIT_COMMIT_HASH=$(git rev-parse HEAD)
  if git_status=$(git status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
    GIT_TREESTATE="clean"
  else
    GIT_TREESTATE="dirty"
  fi
  BUILDDATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
  LDFLAGS="-X github.com/kosmos.io/eps-probe-plugin/pkg/version.gitVersion=${GIT_VERSION} \
                        -X github.com/kosmos.io/eps-probe-plugin/pkg/version.gitCommit=${GIT_COMMIT_HASH} \
                        -X github.com/kosmos.io/eps-probe-plugin/pkg/version.gitTreeState=${GIT_TREESTATE} \
                        -X github.com/kosmos.io/eps-probe-plugin/pkg/version.buildDate=${BUILDDATE}"
  echo $LDFLAGS
}

function util::get_target_source() {
  local target=$1
  for s in "${CLUSTERLINK_TARGET_SOURCE[@]}"; do
    if [[ "$s" == ${target}=* ]]; then
      echo "${s##${target}=}"
      return
    fi
  done
}
