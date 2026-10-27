#!/usr/bin/env bash
# Run a CI Go test suite and report exactly the coverage files it produces.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.."

repo_root="$PWD"
coverage_files=()

covered_test() {
  local profile="$1"
  shift
  go test -coverprofile="$profile" -covermode=atomic "$@"
  local report_path="$PWD/$profile"
  coverage_files+=("${report_path#"$repo_root"/}")
}

covered_web_test() {
  local profile="$1" goroot
  shift
  goroot="$(go env GOROOT)"
  # Go parses -exec as a command line, so preserve quotes around the path.
  GOOS=js GOARCH=wasm covered_test "$profile" "-exec=\"$goroot/lib/wasm/go_js_wasm_exec\"" "$@"
}

require_native_platform() {
  local expected="$1" host target
  host="$(go env GOHOSTOS)"
  target="$(go env GOOS)"
  if [ "$host" != "$expected" ] || [ "$target" != "$expected" ]; then
    echo "This Go coverage suite requires a native $expected runner (host=$host, target=$target)" >&2
    exit 1
  fi
}

case "${1:-}" in
  # These focused suites complement Linux coverage without repeating all tests.
  macos)
    require_native_platform darwin
    covered_test coverage-buildctl-macos.out -v \
      -run '^(TestCommandRunnerRepairsMacOSGoToolchainForChild|TestCommandRunnerRunCommandReturnsEnvironmentError|TestConfigureCurrentMacOSGoToolchainEnv|TestCurrentBuildEnvReturnsIndependentCopy|TestSetupJDKWithHomebrewFixture|TestCompressWasmArtifactsInstallsBrotli|TestBrotliLinuxInstallRejectsMissingReleaseAndTools)$' \
      ./internal/cmd/buildctl/shared ./internal/cmd/buildctl/tool ./internal/cmd/buildctl/runtimecmd
    ;;
  windows)
    require_native_platform windows
    covered_test coverage-buildctl-windows.out -v \
      -run '^Test(ReplaceFileWindows|Windows.*|FetchURLToFileReplacesExistingDestination|SConsEnvironmentCommands|SetupJDKWindowsRequiresManualInstallation)$' \
      ./internal/cmd/buildctl/shared ./internal/cmd/buildctl/tool
    covered_test coverage-engine-windows.out -v \
      -run '^Test(DetectStaleEngineBuildLockProtectsLivePID|EngineBuildLockRecoversExitedPID|EngineLockWaitsForActiveOwner)$' \
      ./internal/cmd/buildctl/engine
    ;;
  web)
    covered_web_test coverage-web.out \
      ./internal/gdengine/binding/web ./pkg/ispx ./internal/coroutine
    ;;
  web-process)
    covered_web_test coverage-web-process.out \
      -run 'UnsupportedPlatform|^TestRunRejectsInvalidInput$|^TestRunCommand|^TestPrepareLinuxRuntimePackAssetsRejectsInvalidInputs$' \
      ./internal/cmd/buildctl/engine ./internal/processsupervisor ./x/xgolauncher
    ;;
  web-interpreter)
    cd cmd/ispx
    covered_web_test coverage-web.out ./...
    ;;
  codegen)
    cd internal/cmd/codegen
    covered_test coverage.out -coverpkg=./... ./...
    ;;
  tools)
    covered_test coverage-driverbundle.out -v ./.github/scripts/driverbundle
    for runtime_tool in manifest resolution version; do
      covered_test "coverage-$runtime_tool.out" -v \
        ".github/scripts/runtime/$runtime_tool.go" ".github/scripts/runtime/${runtime_tool}_test.go"
    done
    ;;
  native)
    profile=coverage.out
    bash .github/scripts/test_go.sh -v -coverprofile="$profile" -covermode=atomic
    coverage_files=("$profile" "cmd/ispx/$profile")
    ;;
  *)
    echo "Unknown Go coverage suite: ${1:-}" >&2
    exit 1
    ;;
esac

if [ -n "${GITHUB_OUTPUT:-}" ]; then
  (IFS=,; echo "coverage=${coverage_files[*]}" >> "$GITHUB_OUTPUT")
fi
