# Web interpreter development

The Web interpreter has two Go modules:

| Module | Purpose |
| --- | --- |
| `cmd/ispx` | Base SPX interpreter, with no Builder AI dependency |
| `cmd/ispx/ai` | Interpreter with Builder AI imports, patches, and generated registrations |

The browser assets and JavaScript tests remain shared under `cmd/ispx/web` and `cmd/ispx/tests`.

Run these commands from the repository root:

```sh
# Build the local interpreter with AI (default).
bash cmd/ispx/build.sh

# Build the base interpreter.
bash cmd/ispx/build.sh --base

# Select the base profile through buildctl and installation.
SPX_ISPX_PROFILE=base make build-wasm
```

`SPX_ISPX_PROFILE` accepts `base` or `ai` and defaults to `ai`, including local installation. An explicit `--base` or `--ai` argument overrides that environment setting. Both variants write `cmd/ispx/ispx.wasm`.

Official GitHub release Web bundles (normal, worker, minigame, and miniprogram) and production and development npm packages use only the base interpreter. Builder builds its own AI-enabled interpreter.

Build selection and failure handling can be tested without downloading Go dependencies:

```sh
python3 cmd/ispx/build_test.py
```

CI generates and validates the base interpreter, including the Web smoke. To test AI locally, select the Go version from `internal/release/runtime.lock.json` using `GOTOOLCHAIN`, put Node on `PATH`, and run:

```sh
cd cmd/ispx/ai
go test ./...
GOOS=js GOARCH=wasm go test -exec="\"$(go env GOROOT)/lib/wasm/go_js_wasm_exec\"" ./...
```

From the repository root, `make generate` generates bindings, interpreter registrations and CLI dependency metadata, then formats all three modules. Use `make generate-runtime` to regenerate only registrations and dependency metadata.

## Switching Builder AI between upstream and a fork

[ai/go.mod](ai/go.mod) is the source of truth for the Builder AI version and any temporary fork replacement. The CLI generates matching dependency metadata from it; do not edit `cmd/spx/internal/command/builderai/version_gen.go` by hand. Both interpreter modules use local SPX replacements for development.

When upgrading SPX before Builder AI supports the new version, first update the fork's SPX requirement and imports to the target version, then push that commit. Keep the AI module path as `github.com/goplus/builder/tools/ai`. A major SPX upgrade requires changing imports such as `/v4` to `/v5` inside the fork; a replacement alone cannot make those types compatible.

From the SPX repository root, select the fork and resolve a commit to a pinned Go module version:

```sh
ai_fork=github.com/joeykchen/builder/tools/ai
ai_fork_ref='<fork-commit-sha>'
ai_fork_version="$(go -C cmd/ispx/ai list -m -f '{{.Version}}' "$ai_fork@$ai_fork_ref")"

go -C cmd/ispx/ai mod edit \
  "-replace=github.com/goplus/builder/tools/ai=$ai_fork@$ai_fork_version"
go -C cmd/ispx/ai mod tidy
make generate-runtime
```

Keep the `require` entry on the official module and use a versioned remote `replace` for the fork. The metadata generator rejects local directory replacements so generated projects can resolve the same dependency on other machines.

Once upstream supports the target SPX version, remove the replacement before updating the requirement:

```sh
ai_upstream_ref='<upstream-commit-or-version>'
go -C cmd/ispx/ai mod edit -dropreplace=github.com/goplus/builder/tools/ai
go -C cmd/ispx/ai get "github.com/goplus/builder/tools/ai@$ai_upstream_ref"
go -C cmd/ispx/ai mod tidy
make generate-runtime
```

After either switch, run the AI native and WASM tests above, `go test ./cmd/spx/internal/command/builderai/...` from the repository root, and `bash cmd/ispx/build.sh --ai`. Commit the module files, generated CLI metadata, and AI registration changes together. Changes to the upstream API may also require adapting the bridge and its tests.
