#!/usr/bin/env bash
set -euo pipefail

# Optional inline config (used only if env vars are not set).
# WARNING: storing PATs in files is sensitive; prefer exporting env vars.
CFG_BASE_URL=""
CFG_PROJECT=""
CFG_PAT=""
CFG_WIT_TYPE=""
CFG_ASSIGNED_TO=""
CFG_INSECURE=""

export TFS_BASE_URL=""
export TFS_PROJECT=""
export TFS_PAT=""
export TFS_WIT_TYPE="Элемент невыполненной работы по продукту"
export TFS_ASSIGNED_TO="${TFS_ASSIGNED_TO:-$CFG_ASSIGNED_TO}"
export TFS_INSECURE="${TFS_INSECURE:-$CFG_INSECURE}"

missing=0
for var in TFS_BASE_URL TFS_PROJECT TFS_PAT TFS_WIT_TYPE; do
  if [[ -z "${!var:-}" ]]; then
    echo "Missing required env: $var" >&2
    missing=1
  fi
done

if [[ $missing -ne 0 ]]; then
  cat <<'USAGE' >&2
Usage:
  export TFS_BASE_URL="https://tfs.example.com/DefaultCollection"
  export TFS_PROJECT="MyProject"
  export TFS_PAT="..."
  export TFS_WIT_TYPE="User Story"
  # export TFS_ASSIGNED_TO="Name<email>"   # optional
  # export TFS_INSECURE=1                  # optional
  ./scripts/run_integration_tests.sh
USAGE
  exit 1
fi

base_url_trimmed="${TFS_BASE_URL%/}"
if [[ "$base_url_trimmed" == */"${TFS_PROJECT}" ]]; then
  echo "Warning: TFS_BASE_URL looks like it includes the project name." >&2
  echo "Expected a collection/organization base URL (project is passed separately)." >&2
fi

cache_root="$(pwd)/.cache"
run_cache="$cache_root/run-$(date +%s%N)"
mkdir -p "$run_cache"/home "$run_cache"/gopath "$run_cache"/gocache "$run_cache"/gomod "$run_cache"/gotmp
export HOME="$run_cache/home"
export GOPATH="$run_cache/gopath"
export GOCACHE="$run_cache/gocache"
export GOMODCACHE="$run_cache/gomod"
export GOTMPDIR="$run_cache/gotmp"

GO_TEST_FLAGS="${GO_TEST_FLAGS:--count=1}"
go test $GO_TEST_FLAGS -tags=integration ./internal/integration -run TestTFSIntegration
