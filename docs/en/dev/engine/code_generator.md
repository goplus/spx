# SPX Binding Code Generation

The generator keeps the Go SPX API, native Godot bindings, Web bridge, and engine implementation synchronized. Generated output is checked into the repository, but must be changed through its declarations and templates.

## 1. Common commands

```sh
make generate-bindings
make generate
```

`generate-bindings` updates binding output only. `generate` also updates runtime registration and formats Go code.

## 2. Pipeline

1. Collect marked public methods from the SPX manager headers.
2. Generate and preprocess the C interface header, then parse its AST.
3. Apply naming, type, and `SPX_BINDING` options.
4. Render native Go, engine-facing, C++, Web JavaScript, and worker templates.
5. Format generated files and compile the affected targets.

The main generator lives under `internal/cmd/codegen`. Its SPX Godot module input and output live at `godot_modules/spx`, independently of the Godot checkout. Set `SPX_MODULE_SRC` to override that module location; relative values resolve from the SPX repository root.

The generator is a nested Go module with a local `replace` back to the repository root. Its `github.com/goplus/spx/v3 v3.0.0` requirement is therefore only the minimum valid v3 module version used to bootstrap `go run`; it is not a release-version declaration and must not be bumped with SPX releases.

## 3. Inputs and outputs

### Inputs

- exported method declarations in `$(SPX_MODULE_SRC)/spx*mgr.h`;
- generator source and templates under `internal/cmd/codegen`;
- SPX module headers and integration files under `godot_modules/spx`.

### Outputs

Generated output includes:

- native engine bindings;
- Web/WASM bridge methods;
- engine implementation and synchronization wrappers;
- exported Go API adapters;
- Godot C++ and injected JavaScript glue;
- Web Worker wrappers.

Files containing `.gen.` in their names and generator-owned bridge sections must not be edited manually.

## 4. Generator structure

The entry point collects marked manager declarations, generates the C interface header, parses its AST, and renders platform templates. Header collection supplies binding metadata; the AST supplies function names and types. Templates own platform-specific syntax but should not redefine API semantics.

## 5. Export rules

### Basic rules

Only declarations selected by the generator's export conventions become bridge methods. Parameter and result types must have a supported ABI representation. Keep public naming stable and make conversions explicit at the engine boundary.

### Native arrays

Pointer-and-length signatures declare caller-provided array buffers. Use `SPX_BINDING(output_count=...)` for fixed-size output or `SPX_BINDING(array_arg=..., elements_per_input=...)` for slice-returning transforms. Both sides must agree on element layout, length, ownership, and lifetime.

Array ABI IDs, the descriptor tag, element names, fixed element widths, and Go/C type mappings are defined once in `generate/common/arrays.go`. The C enum, Web Go `arrays.gen.go`, and the marked array ABI section in `gdspx.util.js` are generated from that definition, including the Go/JS element-size lookup functions. When adding a type, also verify that the native and Web runtimes support its element layout.

### Web caches

Define `Cached<Manager><Method>` functions in `internal/gdengine/binding/web/*_cache.go`. Their arguments match the API, followed by a `func() T` fallback. For example, `CachedInputGetKey(key int64, fallback func() bool) bool` wraps Input’s `get_key`. Functions access shared runtime cache state without constructing cache instances.

The generator discovers these functions and emits the cache call and FFI fallback. Cache policy and boolean adaptation stay in Go; C++ headers need no cache annotations. Missing APIs and invalid fallback signatures fail generation.

### Converted arrays

Unsupported array types require generated conversion code. Prefer conversion at one boundary instead of spreading element-by-element platform handling across callers.

## 6. Adding an interface

1. Add or update the authoritative declaration.
2. Add engine implementation where required.
3. Update generator type rules or templates only if the signature is new.
4. Run `make generate` with the repository-owned module, or set `SPX_MODULE_SRC` explicitly.
5. Review changes in the SPX worktree.
6. Compile native and/or Web targets affected by the interface.
7. Add a behavioral test at the public API level.

## 7. Troubleshooting

If a method is missing, confirm that its declaration is exported and discovered. If it is absent from the parsed model, inspect AST collection and build constraints. If module output is unchanged, verify `SPX_MODULE_SRC`. If only Web fails, compare the generated ABI signature, JavaScript marshalling, and worker wrapper with the native path.

For stale generated files, check the selected SPX module directory and regenerate before debugging hand-written callers. Binding generation uses `SPX_MODULE_SRC` independently of the Godot checkout and Web build mode.

## 8. Verification

Review generated diffs before formatting hides structural mistakes. Run the relevant Go tests, compile the affected Godot target, and for Web changes test the intended mode. Successful generation alone does not prove ABI compatibility.
