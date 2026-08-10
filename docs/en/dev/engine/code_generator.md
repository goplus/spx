# SPX Binding Code Generation

The generator keeps the Go SPX API, native Godot bindings, Web bridge, and engine implementation synchronized. Generated output is checked into the repository, but must be changed through its declarations and templates.

## 1. Common commands

```sh
make generate-bindings
make generate
```

`generate-bindings` updates binding output only. `generate` also updates runtime registration and formats Go code.

## 2. Pipeline

1. Collect exported declarations and binding headers.
2. Parse Go declarations into the generator's intermediate model.
3. Apply export, naming, type, and raw-method rules.
4. Render native Go, engine-facing, C++, Web JavaScript, and worker templates.
5. Format generated files and compile the affected targets.

The main generator lives under `internal/cmd/codegen`. Its SPX Godot module input and output live at `godot_modules/spx`, independently of the Godot checkout. Set `SPX_MODULE_SRC` to override that module location; relative values resolve from the SPX repository root.

## 3. Inputs and outputs

### Inputs

- public and engine-facing declarations under `pkg/spx` and `pkg/spx/pkg/engine`;
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

The entry point loads configuration, discovers declarations, builds an abstract method/type model, and renders platform templates. Header collection establishes the ABI surface. AST parsing supplies Go names, receivers, parameters, results, and annotations. Templates own platform-specific syntax but should not redefine API semantics.

## 5. Export rules

### Basic rules

Only declarations selected by the generator's export conventions become bridge methods. Parameter and result types must have a supported ABI representation. Keep public naming stable and make conversions explicit at the engine boundary.

### `_raw` methods

Raw methods expose a lower-level engine representation for a generated higher-level wrapper. They require special pairing and naming rules; confirm both methods in generated output when adding or changing one.

### Native arrays

Arrays with a directly supported native representation can take a fast bridge path. Both sides must agree on element layout, length, ownership, and lifetime.

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

Stale generated files often indicate that generation ran against a different Godot checkout or with the wrong Web mode. Regenerate before debugging hand-written callers.

## 8. Verification

Review generated diffs before formatting hides structural mistakes. Run the relevant Go tests, compile the affected Godot target, and for Web changes test the intended mode. Successful generation alone does not prove ABI compatibility.
