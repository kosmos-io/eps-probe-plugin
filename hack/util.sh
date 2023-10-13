#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function util::get_version() {
  git describe --tags --dirty --always
}