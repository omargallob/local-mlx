#!/usr/bin/env bash
# Wrapper so `bazel run //tools/lint:golangci-lint -- <args>` runs the hermetic
# golangci-lint binary in the user's workspace (not the runfiles tree).
set -euo pipefail

# --- runfiles.bash initialization v3 ---
set +e
f=bazel_tools/tools/bash/runfiles/runfiles.bash
# shellcheck disable=SC1090
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null ||
	source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null ||
	source "$0.runfiles/$f" 2>/dev/null ||
	source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
	source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
	{
		echo >&2 "ERROR: cannot find runfiles.bash"
		exit 1
	}
set -e
# --- end runfiles.bash initialization v3 ---

bin="$(rlocation _main/tools/lint/golangci-lint-bin)"

cd "${BUILD_WORKSPACE_DIRECTORY:-$PWD}"
exec "$bin" "$@"
