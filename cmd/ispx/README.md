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

Builder AI dependencies and the temporary v4 compatibility replacement are pinned in [ai/go.mod](ai/go.mod). The CLI generates matching metadata for AI-enabled projects without adding AI to the base interpreter. Update these dependencies manually, then run `make generate` and the local tests above. When upstream supports v4, update the AI requirement and remove the fork replacement. Both interpreter modules use local SPX v4 replacements for development.
