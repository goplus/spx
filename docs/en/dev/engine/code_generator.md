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

The entry point collects marked manager declarations, generates the C interface header, parses its AST, and renders platform templates. Header collection supplies binding metadata; the AST supplies function names and types. Templates own platform-specific syntax but should not redefine API semantics. `common/parameters.go` prepares public names, Go types, ABI positions, and array length associations once for all renderers. Header metadata explicitly identifies lowered return parameters, so a user argument named `ret_value` remains an ordinary argument.

## 5. Export rules

### Basic rules

Only declarations selected by the generator's export conventions become bridge methods. Parameter and result types must have a supported ABI representation. Keep public naming stable and make conversions explicit at the engine boundary.

Manager discovery recognizes `Spx*Mgr` classes in `spx*mgr.h`, independently of their base classes. Only marked public declarations are exported. Removing `SpxBaseMgr` inheritance does not remove an interface; renaming the manager still changes its ABI name.

### Ownership and lifecycle bindings

`SPX_BINDING(abi=free_string)` marks a raw ABI deallocator with the signature `void method(GdString value)`. Native C++ calls `SpxAbi::free_return_cstr` directly without resolving an engine or manager. The high-level Go/JS compatibility methods are no-ops because their strings are language-owned values; raw Native return conversion still releases its owned pointer through the original export. `spx_abi.h/.cpp` own allocation, release, and typed array access independently of engine lifetime.

`SPX_BINDING(control=reset)` routes a lifecycle operation directly through `Spx`, preserving the public method name and ABI. Supported targets are `reset` (one `GdInt` argument), `restart`, `pause`, `resume`, `next_frame` (no arguments, `void`) and `is_paused` (no arguments, `GdBool`). These apply to Native and ordinary Web C++ bridges. They cannot be combined with Web overrides or memory-release bindings. Exit and panic notification APIs remain runtime operations.

The generator also emits `spx_callback_defaults.gen.h` from the existing `SpxCallbackInfo` fields and callback typedefs. The engine uses this table of typed no-op callbacks; no separate callback schema or ABI layout change is needed.

### Native arrays

Pointer-and-length signatures declare caller-provided array buffers. Declare fixed output as `SPX_API void write_snapshot(SPX_OUT float out[3]);`: codegen extracts the extent before lowering to a pointer-only ABI, and exposes `WriteSnapshot(out *[3]float32)` in Go. Go rejects nil and Web checks the exact output length before calling C++. Fixed and dynamic buffers share the `float` / `real_t`, `int64_t`, `uint8_t`, and `GdObj` mappings and can be combined with ordinary parameters in a method returning `void`, or `GdBool` when it has `SPX_OUT` parameters. Const arrays are read-only inputs. Fixed arrays require a positive decimal int32 literal extent and no separate length parameter; dynamic slice lengths are checked for int32 overflow before calling the ABI. Only `void` methods with a single fixed `SPX_OUT` array receive the no-argument Web output reader.

For independently sized native arrays, declare `SPX_API GdBool batch_retrieve_positions(const GdObj *objs, int count, SPX_OUT float *out, int out_len);`. Go exposes `BatchRetrievePositions(objs []int64, out []float32) bool` and passes each slice's own length. The generator supports consecutive pointer/length pairs without imposing an input/output ratio. The caller supplies output storage, and the concrete C++ method owns record parsing and capacity checks. For positions, N objects require exactly 2N floats. A larger backing buffer can be sliced to this range. The method validates all arguments before writing, returns false without changing output on failure, and fills the entire range on success. Missing objects produce NaN pairs; empty input and output succeed. This ratio belongs to the position method, not the generator. Const buffers are input-only; writable buffers are copied back when output is valid. The Go caller reuses output storage through the existing per-game `SpriteSyncBuffer.GetPositions` method, growing it only when capacity is insufficient and returning an empty result on failure so stale positions are not applied. Native bindings pass slice pointers directly. Web reuses Wasm storage and copies bytes into the caller's Go output slice without a decoded result allocation; separate Go and engine Wasm memories still require byte copies. Buffer reuse does not cache position values. No C++ `GdArray` wrapper or array-result allocation is used for this path.

For example, `void sample(int mode, const float *values, int count, SPX_OUT float out[3])` becomes `Sample(mode int32, values []float32, out *[3]float32)`. The scalar is passed normally, the dynamic length comes from its slice, and the fixed output needs no length argument. Owned values such as `GdString` can also be mixed with arrays; generated Web cleanup runs even if validation or the native call fails.

Array ABI IDs, the descriptor tag, element names, fixed element widths, and Go/C type mappings are defined once in `generate/common/arrays.go`. The C enum, Web Go `arrays.gen.go`, and the marked array ABI section in `gdspx.util.js` are generated from that definition, including the Go/JS element-size lookup functions. When adding a type, also verify that the native and Web runtimes support its element layout.

### Output-only arrays

Place `SPX_OUT` before an array parameter's type. It expands to nothing in C++ and records direction in generator metadata:

| Declaration | Before a Web call | After a Web call |
| --- | --- | --- |
| `const T *input, int len` | Copy input | No copy back |
| `T *buffer, int len` | Copy existing contents | Copy back |
| `SPX_OUT T *out, int len` / `SPX_OUT T out[N]` | Borrow storage without copying or clearing | Copy back when output is valid |

The caller supplies exactly the number of elements to be written. The callee never reads the old contents and fills every output element. Normal return from a `void` method commits the output. For a `GdBool` method, true commits all output buffers; false skips all Web copy-back and requires the C++ method to leave every writable array unchanged. Since Native passes caller pointers directly, all failure checks must precede the first write. Multiple outputs have independent lengths and share the method's success status; no written-count metadata is needed.

`SPX_OUT` cannot annotate const arrays, scalar parameters, or unsupported element types. Fixed output arrays must have exactly the declared length at the JS entry point too. The no-argument Web reader is generated only for one fixed output with a void result, so it cannot discard a success status or read uninitialized input/output storage.

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
