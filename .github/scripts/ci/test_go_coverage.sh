#!/usr/bin/env bash
# Run a CI Go test suite and report exactly the coverage files it produces.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.."

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
    covered_test coverage-command-macos.out -v \
      -run '^TestResolveAppPathReportsPermissionRepairFailure$' ./cmd/spx/internal/command
    ;;
  windows)
    require_native_platform windows
    covered_test coverage-buildctl-windows.out -v \
      -run '^Test(ReplaceFileWindows|Windows.*|FetchURLToFileReplacesExistingDestination|SConsEnvironmentCommands|SetupJDKWindowsRequiresManualInstallation)$' \
      ./internal/cmd/buildctl/shared ./internal/cmd/buildctl/tool
    covered_test coverage-engine-windows.out -v \
      -run '^Test(DetectStaleEngineBuildLockProtectsLivePID|EngineBuildLockRecoversExitedPID|EngineLockWaitsForActiveOwner)$' \
      ./internal/cmd/buildctl/engine
    covered_test coverage-fileutil-windows.out -v \
      -run '^TestReplaceRegularFile' ./internal/base/fileutil
    covered_test coverage-interpsession-windows.out -v \
      -run '^Test(PrepareSession|ReplaceRootFile.*)$' ./internal/interpsession
    covered_test coverage-runtimebundle-windows.out -v \
      -run '^Test(CacheDefaultPermissionsProtectNestedFilesAndRepair|CacheUsesConfiguredWindowsPermissions|WindowsPrivateDACLIsAppliedAtCreation|AcquireCreatesProtectedWindowsCache|VerifyPrivateSecurityDescriptor.*)$' \
      ./internal/runtimebundle
    ;;
  web)
    covered_web_test coverage-web.out -timeout=1m \
      ./internal/core/project ./internal/gdengine/binding/web ./internal/gdengine/impl ./internal/engine/wrap ./pkg/ispx ./internal/engine/coroutine
    # These lifecycle tests run without the Godot host required by engine Main tests.
    covered_web_test coverage-web-engine.out \
      -run '^Test(RequestExitStopsManagedCallerOnWeb|ResetWebRuntime.*|DrainAndReleaseGame.*)$' \
      ./internal/engine
    ;;
  web-process)
    covered_web_test coverage-web-process.out \
      -run 'UnsupportedPlatform|^Test(RunCommand|MaterializeComponent)|^Test(RunRejectsInvalidInput|PrepareLinuxRuntimePackAssetsRejectsInvalidInputs|ProcessStatusSuccess|RunRejectsInvalidPayload|RunContextRejectsInvalidContext|ContextWriter)$' \
      ./internal/cmd/buildctl/engine ./internal/base/processsupervisor ./pkg/launcher
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
    covered_test coverage-tools.out -v ./.github/cmd/...
    ;;
  native)
    bash .github/scripts/ci/test_go.sh --cover -v -coverprofile=coverage.out -covermode=atomic
    coverage_files=(coverage.out cmd/ispx/coverage.out)
    covered_test coverage-engine-packmode.out -v -tags packmode ./internal/engine
    ;;
  *)
    echo "Unknown Go coverage suite: ${1:-}" >&2
    exit 1
    ;;
esac

# Cross-package profiles repeat blocks across test binaries. Compact before upload.
cd "$repo_root"
# The profile processor runs on the host even when callers export a Web target.
host_os="$(go env GOHOSTOS)"
host_arch="$(go env GOHOSTARCH)"
GOOS="$host_os" GOARCH="$host_arch" go run ./.github/cmd/compact-coverage "${coverage_files[@]}"

if [ -n "${GITHUB_OUTPUT:-}" ]; then
  {
    echo 'coverage<<COVERAGE_PATHS'
    printf '%s\n' "${coverage_files[@]}"
    echo 'COVERAGE_PATHS'
  } >> "$GITHUB_OUTPUT"
fi
