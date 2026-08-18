# [Proposal] XGo Project Runtime Provider v1 and SPX Runtime Integration

> Status: source mode is implemented across the three-repository workspace; published mode and coordinated release are still pending
>
> Scope: `goplus/mod`, `goplus/xgo`, `goplus/spx`

## Summary

This proposal introduces a framework-owned, project-scoped, versioned runtime provider for XGo class projects. The framework declares the provider in its own `gox.mod`; XGo locates and builds it from the application's effective Go module/workspace graph, then delegates `run` or a transactional `build` to the provider. `install` reuses `build`.

XGo implements only generic discovery, identity validation, process management, and output transactions. It contains no SPX, Godot, Engine, PCK, or resource-format special cases. The SPX provider owns interpreted execution, runtime assets, project packaging, and the self-contained launcher.

The current implementation covers source mode for the main module, workspace modules, and local replacements. Engine/PCK can reuse release artifacts validated by a manifest and SHA-256; the bridge must be built from SPX source in the effective graph. Published mode for versioned dependencies still requires support for consuming immutable bridge manifests and the corresponding publication pipeline.

## Problem and goals

SPX is not an ordinary "generate Go, then execute" runtime: it requires a matching Engine Runtime, PCK, interpreter bridge, project resource directory, and an isolated run session. Encoding these rules in XGo would make the generic toolchain depend on a particular framework, and would not guarantee that the SPX module, bridge, and Engine ABI come from a consistent version.

This design provides the following user-facing commands:

```sh
xgo run ./game --headless
xgo build -o ./bin/game ./game
xgo install ./game
```

It also enforces these constraints:

- XGo is unaware of SPX/Godot implementation details;
- the runtime provider and class metadata come from the same effective Go graph;
- `xgo run` does not generate `xgo_autogen.go` or write `.temp`, `.godot`, or build intermediates into the project;
- `xgo build` produces a single-file program that runs offline on the host platform;
- projects without a declared runtime continue entirely through the existing GenGo path;
- once a runtime matches, every subsequent error is terminal and must not silently fall back to GenGo.

## Design principles

1. **Framework owns policy**: the framework decides how to run and package its projects; XGo provides only the lifecycle.
2. **One graph, one identity**: metadata, the provider package, and bridge source are all determined by the same effective module/workspace graph.
3. **Fail closed after match**: the legacy path is allowed only when runtime discovery explicitly reports no match.
4. **Immutable inputs**: critical metadata, release manifests, bundles, and outputs are bound to content digests or file identities.
5. **Project remains read-only**: runtime state is kept strictly separate from user source and resources.
6. **Content-addressed reuse**: identical Engine/bridge bundles are reused across projects, with every cache hit verified.

## Overall architecture

```text
application go.mod / go.work
        |
        | effective Go graph + //xgo:class
        v
framework gox.mod ---- runtime v1 <provider-package>
        |
        v
XGo resolver
  - resolve target
  - resolve class metadata and provenance
  - validate minimum XGo version
  - build provider
        |
        | runtime protocol v1
        v
SPX provider
  - validate request and live file identities
  - acquire Engine/PCK and build bridge
  - run: create an isolated session, then interpret
  - build: generate a launcher with the complete payload embedded
```

The responsibility boundary across the three repositories is:

| Component | Responsibility |
| --- | --- |
| `goplus/mod` | `runtime` metadata, resolved module provenance, shared typed request, and argv codec |
| `goplus/xgo` | graph/target resolution, provider discovery and build, protocol invocation, signal forwarding, and build/install output transactions |
| `goplus/spx` | SPX protocol adapter, Engine/bridge/project bundles, interpretation, caching, and self-contained launcher |

The shared layer defines only provider-neutral data. It does not provide a provider lifecycle SDK or contain SPX domain rules.

## Metadata and version boundaries

SPX uses the following declaration form:

```text
xgo 1.8.0

project main.spx Game github.com/goplus/spx/v3 math
runtime v1 github.com/goplus/spx/v3/cmd/xgoruntime

class -embed *.spx SpriteImpl
pack assets index.json
```

`runtime` applies to the nearest preceding `project`; each project may declare at most one. The syntax must be:

```text
runtime <protocol> <provider-import-path>
```

- `protocol` currently supports only `v1`;
- the provider must be a valid Go import path; relative paths, absolute paths, and `path@version` are not allowed;
- a known but malformed `runtime` directive is an error in both strict and lax parsing;
- the provider module version is not written to `gox.mod`; it is determined uniquely by the application's effective Go graph.

The four version identities are independent:

| Identity | Meaning | Source of truth |
| --- | --- | --- |
| Provider protocol `v1` | Version of the invocation contract between XGo and the provider | `runtime` directive |
| XGo `1.8.0` | Minimum tool capability required to understand and dispatch `runtime v1` | declaring `gox.mod` |
| SPX version/source | Code identity of the provider and bridge | application's effective Go graph |
| Engine Runtime/ABI | Engine, PCK, and interface compatibility | SPX runtime lock and release manifest |

Therefore, an XGo `1.7.5` recorded in the runtime lock only identifies the toolchain used to build that Engine Runtime; it does not mean that XGo `1.7.5` supports runtime providers.

## Effective graph and source identity

XGo uses the standard Go toolchain to obtain the effective build list and stores logical selection separately from replacement source:

- `Selected` is the module path/version selected by MVS;
- `Replace` is the source actually read;
- `Effective` is the final source identity used to read metadata and build the provider.

The same graph policy must flow through metadata discovery, provider validation, and provider build, including `GOWORK`, `-mod`, `-modfile`, and `-overlay`. Dependencies must not be calculated by hand from the original `go.mod`, and the provider must not independently resolve a different graph.

The order and identity of class modules come from the `//xgo:class` markers in the application's effective modfile. Only the main or workspace module containing the target, or a class dependency explicitly marked by the application, can provide a runtime.

Resolved metadata carries all of the following:

- the module provenance that declared the project/runtime;
- the canonical path and SHA-256 of the declaring `gox.mod`/`gop.mod`;
- the minimum XGo version declared by that metadata;
- the path and content digest of the target modfile.

When importing resolved class metadata, XGo validates the target modfile snapshot. Before execution, the provider revalidates the declaration, module source, and every other path it actually uses, but does not reinterpret the metadata. This prevents metadata and the provider from coming from different versions, and prevents critical files from being replaced after discovery.

Vendor mode currently cannot provide equivalent module provenance. Vendor projects without a runtime continue through the legacy path; once a runtime match is confirmed, the request fails explicitly as unsupported.

## Target discovery and dispatch

Runtime discovery occurs after target resolution and before any Dir/PkgPath/Files branch or GenGo invocation.

| Target | v1 behavior |
| --- | --- |
| directory | Supported; scan for a top-level project file |
| single project file | Supported; it must be the only project file in its directory |
| import/package path | Supported; locate it from the caller's effective graph |
| multi-file target | Rejected for runtime projects |
| `...` pattern | Rejected for runtime projects |
| `pkg@version` | Rejected; versions can come only from the current graph |

A directory must correspond to exactly one project file. If no class project exists, or if the project does not declare a runtime, discovery returns `NotHandled`; only that result allows XGo to call the legacy implementation. Any other graph, metadata, protocol, provider-build, or provider-execution error is returned directly to the user.

`run` preserves application argument element boundaries and ordering. When needed, the first `--` after the target acts only as XGo's source/argument separator and is removed by XGo; everything after it is passed to the application unchanged.

`install` reuses `build` semantics and installs into the effective `GOBIN`; when `GOBIN` is empty, it uses `bin` under the first `GOPATH` entry. Runtime v1 accepts only one install target at a time.

## Provider protocol v1

The protocol uses a shared typed request encoded as deterministic argv; it does not use a JSON request file or consume `stdin`. `stdin`, `stdout`, and `stderr` are inherited directly, so interactive and pipeline behavior matches an ordinary command.

The request contains five groups of information:

1. action: `run` or `build`;
2. project snapshot: directory, project file, module root, extension, and optional pack metadata;
3. provider identity: package, selected/replacement provenance, declaration path, and digest;
4. graph/build policy: Go command, work directory, workspace, and allowed flags;
5. action payload: application arguments for `run`, or staging/final output for `build`.

The codec consistently rejects unknown, repeated, missing, incomplete option groups, or action-inapplicable fields. The shared layer performs structural path validation; the provider binds paths to real file identities. If the total protocol argv and environment exceed the platform safety budget, startup fails before the provider is launched.

XGo currently passes only these policies to the runtime provider:

| Type | Supported range |
| --- | --- |
| Graph | `-mod=mod|readonly`, `-modfile`, `-overlay`; `-mod=vendor` is used only to produce an explicit unsupported error |
| Build | `-v`, `-x`, `-work`, `-trimpath=true`, `-buildvcs=false` |

Other flags are reported only after the target has been confirmed as a runtime project, so ordinary projects retain their existing behavior.

## Provider build and process boundaries

Before building the provider, XGo verifies that:

- the package is `main`;
- the package is inside the effective module that declared the runtime;
- the package's selected/replacement/source identity exactly matches the metadata;
- the current XGo satisfies the minimum version declared by the metadata;
- `GOOS/GOARCH` match the host, so the provider cannot be used for cross-compilation.

The provider is built in a private temporary directory under the same graph policy, while preserving the caller's CGO selection. The provider inherits the standard streams; XGo forwards cancellation and platform signals and cleans up the entire child process tree. On Unix, the normal exit code or original signal is preserved. On Windows, a Job Object manages the process tree and interrupts are represented as exit codes. Recursive entry into the same runtime provider is rejected.

`XGO_RUNTIME=off` is an explicit disable switch, but its semantics are "report an error and stop", not "fall back to GenGo".

## SPX source mode

The SPX provider currently accepts these source identities:

- the application itself is the SPX main module;
- SPX is in the current workspace;
- SPX is introduced through an unversioned local replacement.

For each request, the provider builds the interpreter bridge from that effective source with host `CGO_ENABLED=1`, and verifies that the build output still comes from the same module identity. Go build cache reuse is allowed, but the bridge file itself is not persistently cached. Regular versioned dependencies, pseudo-versions, and versioned replacements are currently rejected in published mode; downloadable Engine/PCK assets alone cannot prove that the bridge matches the SPX version and Engine interface.

Engine/PCK assets have two trusted sources:

1. local runtime manifest: explicit or discovered from the SPX source tree, containing runtime/ABI/platform and the SHA-256 of every file;
2. published runtime release: a code-level pin selects a fixed release manifest, from which the matching platform Engine and PCK are acquired.

`$GOPATH/bin` is not used for resource discovery. A file name, existence check, or file size does not establish runtime identity. When local artifacts are absent, a clean checkout can download and verify the published Engine/PCK without running an install workflow first; offline mode permits only a complete cache hit whose verification succeeds.

SPX interpretation uses three independent roots:

| Root | Purpose |
| --- | --- |
| `ProjectDir` | User source and project-level references, read-only |
| `AssetDir` | Project resource root selected by `pack`, read-only |
| `SessionDir` | Engine cwd, temporary configuration, and runtime state; disposable |

The provider controls the `--path` value for `SessionDir`; users cannot override it. Each run creates a new session, prepares the bridge/Engine configuration, and then starts the Engine. The project directory must remain unchanged on both success and failure paths.

## Self-contained build

`xgo build` generates a Go launcher and embeds the complete payload with `go:embed` before linking. The payload contains:

- the Engine executable and PCK;
- the interpreter bridge built from the current graph;
- the canonical project bundle;
- SPX/source identity, host platform, runtime/ABI, component digests, and the complete entry table.

The project bundle uses an allowlist rather than traversing the entire repository: it collects only top-level project source files, optional `.config`, the complete pack directory, and files explicitly referenced by the resource index that remain inside `ProjectDir`. Symlinks, special files, path escapes, oversized inputs, and case/Unicode collisions are rejected. Fixed ordering, timestamps, permissions, and compression policy ensure that identical inputs produce the same project bundle digest.

The generated launcher depends on SPX's public launcher package because generated code is compiled in the user's module graph and cannot import an SPX `internal` package. The launcher itself only validates the payload, materializes components, creates a session, starts the Engine, and reproduces its exit status.

The Darwin payload is finalized before linking; the linked executable is then ad-hoc signed and is not appended to or modified afterward. This signing guarantees Mach-O integrity, not Developer ID signing or notarization.

The provider may write only to the private staging path allocated by XGo. On success, XGo verifies that the artifact is a non-empty, non-symlink host executable, then atomically replaces the final output on the same filesystem. On failure, an existing target must remain unchanged. `install` uses the same transaction and changes only the final directory.

The built launcher does not require Go, XGo, SPX, or a network connection. It first validates the payload and host platform, then materializes the embedded Engine, bridge, and project, and finally runs them in a new session.

## Caching and reuse

The component materialization cache is addressed by `namespace + full digest`; Engine, bridge, and project use separate namespaces:

- different projects using the same runtime reuse the same Engine;
- different launchers can reuse the same bridge or project bundle at execution time;
- different content is never shared even when file names are identical;
- a fresh temporary provider binary and source bridge are produced each time, while compilation naturally reuses the Go build cache; XGo does not maintain another persistent provider-binary cache.

Downloaded files and materialized directories are revalidated on every cache hit against the manifest, type, size, and SHA-256. Tampering that preserves file size is rejected; a damaged entry is repaired under an exclusive lock. First materialization uses a sibling temporary path, complete verification, and atomic rename. Shared/exclusive leases prevent concurrent processes from observing partial state or deleting an entry that is in use.

All launcher resources come from the embedded payload, so the first run does not download anything even when the cache is empty. Automatic quota reclamation is not currently implemented; any future GC must continue to honor leases and must not delete components in use by a running process.

## Trust and failure model

The following inputs are all untrusted: module/workspace metadata, provider argv, environment variables, release manifests, ZIP/payload data, project resource indexes, cache contents, and existing output paths.

All boundaries follow these rules:

- validate canonical paths together with real file identities, rather than comparing strings only;
- parse runtime requests and all manifest formats strictly, rejecting unknown or repeated fields;
- reject ZIP files containing absolute paths, `..`, backslash traversal, duplicates/collisions, symlinks, device files, or compression bombs;
- verify file identity before and after reading; consume critical large files through an already-open handle;
- terminate on any identity, digest, runtime, ABI, platform, or source mismatch;
- manage provider, Engine, and launcher child processes under a supervisor; cancellation must not leave child processes behind;
- never fall back from the runtime path to native/GenGo.

## Current boundaries

- XGo `1.8.0` is the runtime-provider capability baseline; earlier versions are unsupported and cannot rely on the old parser to provide a reliable upgrade hint;
- host desktop only: Darwin amd64/arm64, Linux amd64, and Windows amd64;
- `xgo test`, Web, Android, iOS, and `GOOS/GOARCH` cross-compilation are unsupported;
- runtime vendor mode, multiple runtime targets, and arbitrary Go build flags are unsupported;
- SPX requires a separate project pack directory;
- published SPX module mode is not enabled yet; outside an SPX checkout, specifying only a published SPX version does not activate the current provider;
- the launcher contains project source and resources and does not provide source confidentiality;
- launcher size is dominated by Engine/PCK/bridge, and the complete executable is not guaranteed to be bit-for-bit reproducible;
- XGo's public `tool.RunDir/BuildDir/InstallDir` APIs retain their existing semantics; runtime dispatch is currently a CLI capability.

## Release order

Fully enabling published mode requires one coordinated release:

1. Publish `goplus/mod` with runtime metadata, provenance, and the protocol codec;
2. Update XGo to depend on that version, then publish a version containing runtime-provider support;
3. Complete SPX consumption of bridge manifests for published identities, and publish verifiable Engine, PCK, bridge, and immutable manifests for the target runtime/ABI;
4. Verify public downloads, offline cache, source mode, and the self-contained launcher;
5. Finally publish the new SPX module version containing the `runtime v1` declaration.

Until step 3 is complete, SPX must fail explicitly for published module identities. It must not borrow a local bridge or infer compatibility from runtime file names alone.

## Acceptance criteria

- run/build/install behavior for ordinary XGo projects remains unchanged;
- directory, single-file, and package targets for runtime projects do not execute GenGo after discovery;
- metadata, provider, and bridge for workspace and local-replacement projects always come from the same effective graph;
- a clean SPX checkout can run and build without preinstalled resources in `$GOPATH/bin`;
- cache miss, cache hit, offline, concurrency, kill-recovery, and same-size tampering are all covered by tests;
- run fully preserves argv, stdin/stdout/stderr, and platform exit semantics; Unix signals are reproduced, Windows interrupts return 130, and the project is not modified;
- build/install failures do not corrupt an existing output;
- the launcher completes its first run with an empty cache, no toolchain, and no network;
- real host artifacts for Darwin, Linux, and Windows pass platform smoke tests.
