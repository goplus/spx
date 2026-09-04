# [Proposal] XGo Project Driver v1 and SPX Runtime Integration

> Status: source mode and the published bundle path are implemented; an exact SPX module version selects its published driver directly, with SPX/runtime versions and asset integrity checked before use; immutable-release rollout and native Windows evidence remain production acceptance gates
>
> Scope: `goplus/mod`, `goplus/xgo`, `goplus/spx`
>
> Canonical driver-neutral wire and provenance contract: `goplus/mod/driverprotocol/spec-v1.md` (must land in mod before the coordinated dependency release)

## Summary

This proposal introduces a framework-owned, project-scoped, versioned project driver for XGo class projects. The framework declares the driver in its own `gox.mod`; XGo locates and builds it from the application's effective Go module/workspace graph, then delegates `run` or a transactional `build` to the driver. `install` reuses `build`.

XGo implements only generic discovery, identity validation, process management, and output transactions. It contains no SPX, Godot, Engine, PCK, or resource-format special cases. The SPX driver owns interpreted execution, runtime assets, project packaging, and the self-contained launcher.

The current implementation covers source mode for the main module, workspace modules, and local replacements. Published mode uses a combined driver bundle v1: the exact canonical SPX module version selects driver assets in the SPX Release whose tag is exactly `spx_version` (for example, `v3.2.4`), and each host ZIP contains exactly the Engine, PCK, and interpreter bridge. This ZIP is an independent driver asset, but not an independent GitHub Release, and not an existing standalone Engine/PCK runtime ZIP. Its manifest's `spx_version` must match the selected module version and `runtime_version` must match the current module lock; bundle sizes and SHA-256 values are still verified, while bundle identity and URLs are not derived from `runtime_version`.

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
- the project driver and class metadata come from the same effective Go graph;
- `xgo run` does not generate `xgo_autogen.go` or write `.temp`, `.godot`, or build intermediates into the project;
- `xgo build` produces a single-file program that runs offline on the host platform;
- projects without a declared driver continue entirely through the existing GenGo path;
- once a driver matches, every subsequent error is terminal and must not silently fall back to GenGo.

## Design principles

1. **Framework owns policy**: the framework decides how to run and package its projects; XGo provides only the lifecycle.
2. **One graph, one identity**: metadata and the driver package come from the same effective module/workspace graph; source mode also builds its bridge from that graph, while published mode uses the bundle for the exact module version.
3. **Fail closed after match**: the legacy path is allowed only when driver discovery explicitly reports no match.
4. **Immutable inputs**: critical metadata, release manifests, bundles, and outputs are bound to content digests or file identities.
5. **Project remains read-only**: runtime state is kept strictly separate from user source and resources.
6. **Content-addressed reuse**: identical Engine/bridge bundles are reused across projects, with every cache hit verified.

## Overall architecture

```text
application go.mod / go.work
        |
        | effective Go graph + //xgo:class
        v
framework gox.mod ---- driver v1 <driver-package>
        |
        v
XGo resolver
  - resolve target
  - resolve class metadata and provenance
  - validate the declaring module's XGo requirement
  - build driver
        |
        | driver protocol v1
        v
SPX driver
  - validate request and live file identities
  - acquire source runtime or the published driver bundle
  - build the bridge in source mode
  - run: create an isolated session, then interpret
  - build: generate a launcher with the complete payload embedded
```

The responsibility boundary across the three repositories is:

| Component | Responsibility |
| --- | --- |
| `goplus/mod` | `driver` metadata, resolved module provenance, shared typed request, and argv codec |
| `goplus/xgo` | graph/target resolution, driver discovery and build, protocol invocation, signal forwarding, and build/install output transactions |
| `goplus/spx` | SPX protocol adapter, Engine/bridge/project bundles, interpretation, caching, and self-contained launcher |

The shared layer defines only driver-neutral data. It does not provide a driver lifecycle SDK or contain SPX domain rules.

## Metadata and version boundaries

SPX uses the following declaration form:

```text
xgo 1.8.0

project main.spx Game github.com/goplus/spx/v3 math
driver v1 github.com/goplus/spx/v3/cmd/xgodriver

class -embed *.spx SpriteImpl
pack assets index.json
```

`driver` applies to the nearest preceding `project`; each project may declare at most one. The syntax must be:

```text
driver <protocol> <driver-import-path>
```

- `protocol` currently supports only `v1`;
- the driver must be a valid Go import path; relative paths, absolute paths, and `path@version` are not allowed;
- a known but malformed `driver` directive is an error in both strict and lax parsing;
- the driver module version is not written to `gox.mod`; it is determined uniquely by the application's effective Go graph.

The five version and capability dimensions are independent and are combined only for compatibility checks:

| Identity | Meaning | Source of truth |
| --- | --- | --- |
| Driver protocol `v1` | Version of the invocation contract between XGo and the driver | `driver` directive |
| Project-driver v1 capability baseline | Earliest XGo version that understands and dispatches `driver v1`, currently `1.8.0` | XGo protocol implementation and compatibility policy |
| Declaring module's XGo requirement | Minimum XGo version needed by that framework release for its metadata or tool features | `xgo` directive in the declaring `gox.mod`/`gop.mod` |
| SPX version/source | Code identity of the driver and bridge | effective Go graph in source mode; selected exact module version and equal `spx_version` in the driver manifest in published mode |
| Engine Runtime/ABI | Engine, PCK, and interface compatibility | runtime version selected by the SPX runtime lock and equal `runtime_version` in runtime/driver manifests |

The `driver v1` and `xgo` directives do not redefine each other. If a later SPX release raises `xgo` to `1.9.0`, only that SPX release requires XGo 1.9.0 features. The v1 protocol baseline remains 1.8.0, while the effective minimum for that SPX project is `max(1.8.0, 1.9.0) = 1.9.0`; this must not be described as “driver v1 requires XGo 1.9.0.”

Therefore, an XGo `1.7.5` recorded in the runtime lock only identifies the toolchain used to build that Engine Runtime; it does not mean that XGo `1.7.5` supports project drivers.

## Effective graph and source identity

XGo uses the standard Go toolchain to obtain the effective build list and stores logical selection separately from replacement source:

- `Selected` is the module path/version selected by MVS;
- `Replace` is the source actually read;
- `Effective` is the final source identity used to read metadata and build the driver.

Except for the isolated `pkg@version` classification probe, the same supported graph
policy must flow through metadata discovery, driver validation, and driver build,
including `GOWORK`, `-mod`, and `-modfile`. Dependencies must not be calculated by hand
from the original `go.mod`, and the driver must not independently resolve another graph.
A missing host Go command, lexical GOFLAGS parse failure, GOWORK query/canonicalization
failure, or a non-`not exist` graph-input inspection error is terminal before
classification.

A separate class of errors may be deferred so v1-only flag policy does not change an
ordinary XGo command: a lexically separable but unsupported flag/value, or a missing
`-modfile`/`-overlay`, may be removed from a read-only metadata compatibility probe and
recorded as the original error. That probe answers only whether metadata declares a
driver. It never validates, builds, or executes a driver and is not a substitute graph
that may reach a consumer. With no match, legacy handling receives the original flags;
with a match, the recorded driver-policy error is returned first. A valid `-overlay`
may participate in the classification view but is likewise rejected after a positive
match. Project-driver v1 does not define an overlay-aware project snapshot, so ordinary
projects retain their legacy behavior.

The order and identity of class modules come from the `//xgo:class` markers in the application's effective modfile. Only the main or workspace module containing the target, or a class dependency explicitly marked by the application, can provide a driver.

Resolved metadata carries all of the following:

- the module provenance that declared the project/driver;
- the canonical path and SHA-256 of the declaring `gox.mod`/`gop.mod`;
- the declaring module's XGo version requirement;
- the path and content digest of the target modfile.

When importing resolved class metadata, XGo validates the target modfile snapshot. Before execution, the driver revalidates the declaration, module source, and every other path it actually uses, but does not reinterpret the metadata. This prevents metadata and the driver from coming from different versions, and prevents critical files from being replaced after discovery.

Vendor mode currently cannot provide equivalent module provenance. When the active module has no external class markers, XGo can still classify the target from that module's own metadata: a non-driver target continues through the legacy path, while a driver match fails explicitly as unsupported. If the effective modfile contains an external class marker, v1 fails closed before classification because standard Go vendor data may omit the dependency's `gox.mod`/`gop.mod`; consulting a live replacement would violate vendor snapshot identity. Preserving legacy behavior for those graphs requires a future XGo-owned vendor manifest with complete metadata identities and digests.

## Target discovery and dispatch

Driver discovery occurs after target resolution and before any Dir/PkgPath/Files branch or GenGo invocation.

Once a target is confirmed as driver-backed, the driver owns any driver-specific code generation in a private isolated work directory; v1 defines no XGo pre-generated artifact or generated-output handoff protocol.

| Target | v1 behavior |
| --- | --- |
| directory | Supported; scan for a top-level project file |
| single project file | Supported; it must be the only project file in its directory |
| import/package path | Supported; locate it from the caller's effective graph |
| multi-file target | Rejected for driver-backed projects |
| `...` pattern | Rejected for driver-backed projects |
| `pkg@version` | Classify only the requested version in a temporary `GOWORK=off`, `-mod=mod` graph; an ordinary target remains legacy, while a driver match is rejected explicitly |

A directory must correspond to exactly one project file. If no class project exists, or if the project does not declare a driver, discovery returns `NotHandled`; only that result allows XGo to call the legacy implementation. Any other graph, metadata, protocol, driver-build, or driver-execution error is returned directly to the user.

`run` preserves application argument element boundaries and ordering. When needed, the first `--` after the target acts only as XGo's source/argument separator and is removed by XGo; everything after it is passed to the application unchanged.

`install` reuses `build` semantics and installs into the effective `GOBIN`; when `GOBIN` is empty, it uses `bin` under the first `GOPATH` entry. Driver v1 accepts only one install target at a time.

## Driver protocol v1

The protocol uses a shared typed request encoded as deterministic argv; it does not use a JSON request file or consume `stdin`. `stdin`, `stdout`, and `stderr` are inherited directly, so interactive and pipeline behavior matches an ordinary command.

The request contains five groups of information:

1. action: `run` or `build`;
2. project snapshot: directory, project file, module root, extension, and optional pack metadata;
3. driver identity: package, selected/replacement provenance, declaration path, and digest;
4. graph/build policy: Go command, work directory, workspace, and allowed flags;
5. action payload: application arguments for `run`, or staging/final output for `build`.

The codec consistently rejects unknown, repeated, missing, incomplete option groups, or action-inapplicable fields. The shared layer performs structural path validation; the driver binds paths to real file identities. If the total protocol argv and environment exceed the platform safety budget, startup fails before the driver is launched.

XGo currently passes only these policies to the project driver:

| Type | Supported range |
| --- | --- |
| Graph | `-mod=mod|readonly`, `-modfile`; `-overlay` is used only to produce an explicit unsupported error after a driver match; `-mod=vendor` permits conservative discovery from the active module only and rejects driver matches or indeterminate external class metadata |
| Build | `-v`, `-x`, `-work`, `-trimpath=true`, `-buildvcs=false` |

Explicit CLI defaults `-trimpath=false` and `-buildvcs=auto` are equivalent to omission;
XGo accepts and normalizes them away instead of placing them on the wire. Other semantic
or unknown flags remain errors after a driver match.

Other flags are reported only after the target has been confirmed as driver-backed, so ordinary projects retain their existing behavior.

The SPX driver does not force `-mod=mod` to `-mod=readonly` merely to keep the project
read-only. Workspace mode makes a stable private copy of `go.work/go.work.sum`.
With `GOWORK=off` and `-mod=mod`, it copies the active modfile and sidecar sum to
private `graph.mod/graph.sum` files and makes relative local replacements absolute
against the active module root, even when an explicit modfile lives elsewhere.
Subsequent SPX-owned Go commands may update only those private copies; the verifier still pins
the original graph files and the target modfile declared by the protocol.

## Driver build and process boundaries

Before building the driver, XGo verifies that:

- the package is `main`;
- the package is inside the effective module that declared the driver;
- the package's selected/replacement/source identity exactly matches the metadata;
- the current XGo satisfies `max(1.8.0, the declaring module's xgo requirement)`;
- `GOOS/GOARCH` match the host, so the driver cannot be used for cross-compilation.

The driver is built in a private temporary directory under the same graph policy while
preserving the caller's CGO selection, and it inherits the standard streams. The
external driver/Engine chain has one command boundary that owns host signals. Inner
supervisors consume cancellation, including the original signal as its cause, and clean
up the complete chain without subscribing to those signals again. A Unix driver runs
in its own process group; a Windows driver is assigned to a kill-on-close Job before
executing user code. Synchronous Go helpers used for discovery and driver construction
are context-bound and fully `Wait`ed, but v1 does not extend a second process-group/Job
tree contract to those helpers. Once cancellation has been observed, a child that exits
successfully during shutdown cannot turn the request into success. Unix preserves the
normal exit code or original signal; Windows represents a host interrupt as `130`.
Project-driver v1 rejects every nested driver dispatch, including dispatch to a
different driver.

`XGO_DRIVER=off` is an explicit disable switch, but its semantics are "report an error and stop", not "fall back to GenGo".

## SPX source and published modes

The SPX driver currently accepts these source identities:

- the application itself is the SPX main module;
- SPX is in the current workspace;
- SPX is introduced through an unversioned local replacement.

These identities always use source mode. A main/workspace module or an unversioned local replacement does not switch to published mode merely because the selected module has a version.

The portable driver snapshot contains only project-rooted files. It therefore rejects legacy `extasset` configuration explicitly. This restriction is scoped to the project-driver path; existing SPX run, native, export, and pack commands retain their legacy external-asset behavior.

The `.config` contract is bound to the bytes actually consumed. The driver snapshots its absence or presence and SHA-256, revalidates the original path before handoff, and then supplies run/build only the captured bytes instead of reopening the project copy.

In source mode, each request builds the interpreter bridge from that effective source with host `CGO_ENABLED=1`, and verifies that the build output still comes from the same module identity. Go build cache reuse is allowed, but the bridge file itself is not persistently cached.

Published mode never builds or borrows a bridge from the effective graph. It accepts only the canonical module `github.com/goplus/spx/v3` at an exact canonical release version with no replacement. Pseudo-versions, versioned replacements, and foreign module paths fail closed. The driver locates `driver-manifest.json` and the host ZIP assets in the SPX Release whose tag exactly equals the selected module version and validated `spx_version`, not in an existing standalone runtime ZIP. The manifest's `spx_version` must equal the selected module version and `runtime_version` must equal the runtime version selected by the current module lock; the manifest and ZIP are still strictly validated for schema, host, entry names, sizes, and SHA-256. Each host ZIP must contain exactly Engine, PCK, and bridge, with missing, extra, duplicate, or mismatched entries rejected. A verified ZIP is reused through the content-addressed cache; offline mode succeeds only on a complete verified cache hit.

Runtime inputs are selected by mode:

1. source mode: an explicit local override wins; otherwise the `runtime-v<runtime_version>` release selected by the lock is tried first and its manifest must declare the same runtime version; an exact-version local source/GOPATH runtime is used only when that release is unavailable;
2. published mode: the exact SPX module version selects the driver host ZIP containing all three components from the Release whose tag is exactly that version and `spx_version`. It does not select a driver bundle by `runtime_version` or perform a separate Engine/PCK lookup.

The three Engine-scoped manifests have distinct names and lifetimes: `engine-source-manifest.json` is the local source-mode Engine/PCK input, `engine-component-manifest.json` is generated under `engine/` in a launcher payload, and `engine-acquisition-manifest.json` records how an Engine/PCK pair entered the internal Engine cache. The Runtime Release-level manifest remains `runtime-manifest.json`; its lock entry, Release asset name, and URL contract are unchanged.

Changing unreleased SPX source does not create a published-driver-asset dependency. Main/workspace/local-replace projects remain in source mode, so an unchanged `runtime_version` keeps reusing the same published Engine/PCK while the bridge is rebuilt from the current source. Only an exact published module version requires its driver manifest and host bundle from the SPX Release; release CI must publish the module tag only after those assets pass verification.

Here “source mode” means only main/workspace/local replace. An external demo with `require github.com/goplus/spx/v3 vX.Y.Z //xgo:class` remains in published mode even after Go has downloaded its source into the module cache, and therefore uses the driver assets from the `vX.Y.Z` SPX Release. The module cache is not a mutable source workspace or an implicit bridge-build fallback.

A malformed manifest or any version, size, digest, or host mismatch fails closed in both modes before a runtime asset is used.

`$GOPATH/bin` is not used to satisfy published mode. Published identity comes from the selected module/lock versions and their corresponding declarations in the manifest; a file name, existence check, or file size alone cannot establish content integrity. When local artifacts are absent, a clean source checkout can download and verify the published Engine/PCK without running an install workflow first; published mode downloads and verifies its combined driver ZIP. Offline mode permits only a complete cache hit whose verification succeeds.

SPX interpretation uses three independent roots:

| Root | Purpose |
| --- | --- |
| `ProjectDir` | User source and project-level references; read-only except for XGo's designated staging file |
| `AssetDir` | Project resource root selected by `pack`, read-only |
| `SessionDir` | Engine cwd, temporary configuration, and runtime state; disposable |

The driver controls the `--path` value for `SessionDir`; users cannot override it. Each
run creates a new session, prepares the bridge/Engine configuration, and then starts the
Engine. Driver-owned preparation and cleanup write only XGo's designated staging file;
writes by Engine or application logic are outside the v1 sandbox guarantee.

## Self-contained build

`xgo build` generates a Go launcher and embeds the complete payload with `go:embed` before linking. The payload contains:

- the Engine executable and PCK;
- the source-mode bridge built from the current graph, or the verified bridge from the published driver bundle;
- the canonical project bundle;
- SPX/source identity, host platform, runtime/ABI, component digests, and the complete entry table.

The project bundle uses an allowlist rather than traversing the entire repository: it collects only top-level project source files, optional `.config`, the complete pack directory, and files explicitly referenced by the resource index that remain inside `ProjectDir`. Symlinks, special files, path escapes, oversized inputs, and case/Unicode collisions are rejected. Fixed ordering, timestamps, permissions, and compression policy ensure that identical inputs produce the same project bundle digest.

The generated launcher depends on SPX's public launcher package because generated code is compiled in the user's module graph and cannot import an SPX `internal` package. The launcher itself only validates the payload, materializes components, creates a session, starts the Engine, and reproduces its exit status.

The Darwin payload is finalized before linking; the linked executable is then ad-hoc signed and is not appended to or modified afterward. This signing guarantees Mach-O integrity, not Developer ID signing or notarization.

The driver may write only inside the private staging directory allocated by XGo and must
produce its target at the designated staging path. On success, XGo validates and commits
only that target; other temporary or diagnostic files do not participate in commit and
are removed with the staging directory. The target must be a non-empty, non-symlink host
executable, then XGo commits the final output with a same-filesystem replacement. Before
that commit point, driver failure leaves an existing target unchanged. Atomic visibility
and replacement of an existing target are guaranteed only to the extent provided by the
host platform and filesystem; real Windows replacement and crash/recovery behavior remains
a host-CI requirement. `install` uses the same transaction and changes only the final directory.

The built launcher does not require Go, XGo, SPX, or a network connection. It first validates the payload and host platform, then materializes the embedded Engine, bridge, and project, and finally runs them in a new session.

## Caching and reuse

The component materialization cache is addressed by `namespace + full digest`; driver, Engine, bridge, and project use separate namespaces:

Published acquisition first verifies and caches the complete combined ZIP in the driver namespace. Only after that verification may launcher execution materialize and reuse Engine, bridge, and project by component digest; the component cache never bypasses the bundle trust boundary.

- different projects using the same runtime reuse the same Engine;
- different launchers can reuse the same bridge or project bundle at execution time;
- different content is never shared even when file names are identical;
- source mode produces a fresh temporary driver binary and bridge each time, while compilation naturally reuses the Go build cache; published mode reuses only verified bundle components.

Downloaded files and materialized directories are revalidated on every cache hit against
the manifest, type, size, and SHA-256. With the manifest unchanged, same-size tampering
of a bundle or component is rejected; a damaged entry is repaired under an exclusive
lock. First materialization uses a sibling temporary path, complete verification, and a
same-filesystem rename. Atomic publication is relied on only within the guarantees of
the host platform and filesystem; real Windows publish/repair and crash-recovery
scenarios remain host-CI requirements. Shared/exclusive leases prevent concurrent
processes from observing partial state or deleting an entry that is in use.

All launcher resources come from the embedded payload, so the first run does not download anything even when the cache is empty. Automatic quota reclamation is not currently implemented; any future GC must continue to honor leases and must not delete components in use by a running process.

## Trust and failure model

The following inputs are all untrusted: module/workspace metadata, driver argv, environment variables, release manifests, ZIP/payload data, project resource indexes, cache contents, and existing output paths. The selected framework driver itself is trusted executable code running with XGo's OS privileges. Project-read-only and output-transaction guarantees constrain a conforming driver and isolate accidental failure; v1 is not an access-control sandbox against a malicious driver.

Published acquisition trusts the canonical `goplus/spx` GitHub Release endpoint over HTTPS and the repository controls that authorize that Release. The downloaded manifest is the integrity root for its bundle and component SHA-256 values; those hashes prove consistency with the manifest, but do not independently authenticate its publisher or cryptographically bind it to the Go module source. A raw manifest in the offline cache has no second external digest; its cached bytes are the local integrity root, and v1 does not claim resistance to a local attacker that can replace both that root and its components. Project-driver v1 does not currently define a separate signature, transparency proof, or source-embedded manifest digest. Deployments that require a stronger supply-chain identity must add such a signed binding as a versioned contract rather than treating the existing checksums as publisher authentication.

Production publication additionally requires [GitHub Immutable Releases](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases) for new public runtime and SPX releases. Drafts remain mutable while assets are assembled; after publication, the tag and assets must be immutable and carry GitHub's release attestation. This is a publication-side gate: v1 acquisition and offline caches continue to verify manifest and asset hashes, and do not transport or revalidate GitHub attestations. Historical mutable releases cannot satisfy this gate for project-driver v1 production.

All boundaries follow these rules:

- validate canonical paths together with real file identities, rather than comparing strings only;
- parse driver requests and all manifest formats strictly, rejecting unknown or repeated fields;
- reject ZIP files containing absolute paths, `..`, backslash traversal, duplicates/collisions, symlinks, device files, or compression bombs;
- verify file identity before and after reading; consume critical large files through an already-open handle;
- terminate on any version, digest, runtime, ABI, or platform mismatch;
- manage driver, Engine, and launcher child processes under a supervisor; cancellation must not leave child processes behind;
- never fall back from the driver path to native/GenGo.

## Current boundaries

- XGo `1.8.0` is the project-driver capability baseline; earlier versions are unsupported and cannot rely on the old parser to provide a reliable upgrade hint;
- host desktop only: Darwin amd64/arm64, Linux amd64, and Windows amd64;
- `xgo test`, Web, Android, iOS, and `GOOS/GOARCH` cross-compilation are unsupported;
- vendor mode, overlays, multiple driver-backed targets, and arbitrary Go build flags are unsupported;
- SPX requires a separate project pack directory;
- an explicit Windows `-o` name is preserved exactly; only default, directory, and install outputs receive `.exe`;
- published mode accepts only the canonical SPX module at an exact release version and a host bundle in the Release whose tag exactly equals `spx_version`, with `spx_version` and `runtime_version` respectively matching the module and lock; missing Release assets, a manifest version mismatch, or invalid release input fails closed;
- the launcher contains project source and resources and does not provide source confidentiality;
- launcher size is dominated by Engine/PCK/bridge, and the complete executable is not guaranteed to be bit-for-bit reproducible;
- build/install guarantees a private staging/commit transaction for one invocation, but provides no cross-process lock for concurrent writes to the same final path;
- XGo's public `tool.RunDir/BuildDir/InstallDir` APIs retain their existing semantics; driver dispatch is currently a CLI capability.

## Release order

`goplus/mod` metadata/provenance/codec and XGo project-driver support are prerequisites.
First publish a mod version containing the complete `driverprotocol` v1 contract,
including target-modfile identity; then bump the XGo/SPX module dependencies and run
their standalone `GOWORK=off` tests. Production publication then advances
runtime→driver build→SPX Release assembly and publication in one `publish-release` run,
keeping the driver bundle independent of `runtime_version`:

Before the first project-driver v1 production publication, repository administrators must
enable GitHub Immutable Releases. The pipeline may assemble a mutable draft, but every
newly published runtime or SPX Release must become immutable before it can satisfy the
release gate.

1. **Runtime stage**: resolve the `runtime-v<runtime_version>` selected by the lock. Reuse a same-version public release only when it was published under the immutable-release gate. If an existing same-version release is historical and mutable, advance the runtime lock/version before building, verifying, and publishing new immutable runtime assets.
2. **Driver asset stage**: build one ZIP per supported host from the exact SPX release source and generate the manifest. Each ZIP must contain exactly Engine, PCK, and bridge, with names, sizes, and SHA-256 values frozen in the manifest.
3. **SPX Release stage**: combine the product assets, driver manifest, and four host ZIPs into one draft whose tag is exactly `spx_version`, and generate one `SHA256SUMS` covering every asset. After all assets pass verification, publish this single Release for the exact canonical SPX module tag/version; before publication, validate exact-module download, strict manifest/ZIP verification, cache/offline behavior, source mode, and the self-contained launcher.

The unified `publish-release` state machine runs all three stages automatically; the driver stage only builds and verifies assets and never creates or publishes a second Release. The flow does not pause between stages to generate a file or require another commit. Runtime selection identity comes only from the lock's `runtime_version`; reuse separately satisfies the immutable-publication and manifest/asset-integrity gates. Driver asset selection uses only the selected `spx_version` and `runtime_version`. These gates do not create another release identity.

The driver bundle URL and identity come only from the SPX Release whose tag exactly equals the selected module version and `spx_version`, never from `runtime_version`; the manifest runtime version only confirms that the bundle carries Engine/PCK for the current lock. If the Release's driver assets are incomplete or either manifest version differs, published mode must fail explicitly; it must not borrow a local bridge or infer compatibility from runtime file names.

## Acceptance criteria

- run/build/install behavior for ordinary XGo projects remains unchanged;
- directory, single-file, and package targets for driver-backed projects do not execute GenGo after discovery;
- metadata, driver, and bridge for workspace and local-replacement projects always come from the same effective graph;
- a clean SPX checkout can run and build without preinstalled resources in `$GOPATH/bin`;
- release validation must cover published-bundle cache miss/hit, offline, concurrency, kill-recovery, and same-size tampering; Windows host CI is a prerequisite for real publish/replace and crash-recovery coverage;
- a canonical released SPX module downloads the `driver-manifest.json` and host ZIP selected by its exact module version, requires their SPX/runtime versions to match the module and lock respectively, and rejects pseudo-versions, versioned replacements, foreign modules, malformed manifests, and ZIPs other than exactly Engine/PCK/bridge;
- run fully preserves argv, stdin/stdout/stderr, and platform exit semantics; Unix signals are reproduced, Windows interrupts return 130, and the project is not modified;
- build/install failures do not corrupt an existing output;
- the self-contained launcher embeds Engine, PCK, bridge, and project, and completes its first run with an empty cache, no toolchain, and no network;
- real Darwin, Linux, and Windows host artifacts pass platform smoke tests before publication;
- every runtime/SPX Release newly published under the project-driver v1 contract is immutable, and native Windows CI passes descendant cleanup and transactional output tests.

## Normative v1 Contract

The driver-neutral metadata, argv, schema, and provenance rules are canonically owned by
`goplus/mod/driverprotocol/spec-v1.md`, which must land before the coordinated dependency
release.
All driver-neutral metadata, invocation, codec, provenance, and transport-budget
restatements below are informative summaries; the canonical spec wins if they differ.
The XGo dispatch and SPX runtime additions remain normative. A driver-neutral wire or
provenance change must update the canonical spec before the repositories are synchronized;
XGo/SPX-only dispatch or runtime changes are owned by this proposal. Unknown fields are
never silently ignored.

### Normative terms and conformance

The terms MUST, MUST NOT, SHOULD, and MAY are normative. Only text explicitly marked
as an informative summary, example, explanation, or implementation note is non-normative. A v1
implementation conforms only when all three layers conform:

1. **Codec layer:** the producer emits one canonical argv; the consumer accepts the
   equivalent argv forms allowed here and rejects unknown, duplicate, missing, or
   invalidly combined fields.
2. **Dispatcher layer:** XGo derives the target, metadata, and driver identity from one
   effective graph and enforces the `NotHandled`/matched boundary, version floor, and
   output transaction.
3. **Driver layer:** SPX rebinds live identity before using a path, graph, or published
   asset and obeys the source/published-mode, project-read-only, process, and cache
   contracts.

Passing a lower layer does not replace a higher-layer check: an absolute/clean path at
the codec layer does not prove existence, a resolver snapshot does not authorize a
consumer to skip live revalidation, and a manifest checksum is not a publisher
signature. Numeric limits, orderings, digest formulas, and exit statuses explicitly
identified here as wire/archive contracts are v1 interoperability requirements.
Unspecified internal graph limits and fields in acquisition/component/cache manifests
or payload internals may evolve without a protocol bump when external selection,
integrity, and launch behavior do not change.

### Metadata and capability negotiation

The `gox.mod`/`gop.mod` parser accepts a syntactically valid
`driver vN <package>` with `N >= 1` and preserves the protocol value in resolved
metadata; the parser does not infer dispatcher capability. The XGo v1 dispatcher
executes only `v1`. Once a target's project metadata contains any `driver`
declaration, it is a driver match: `v2` or later must produce a terminal
`unsupported driver protocol` error, never masquerade as `NotHandled` and enter
GenGo. A future dispatcher may advertise a new protocol capability only after it
implements that protocol's argv, process, and transaction contracts. Raising the
declaring module's `xgo` directive raises the minimum XGo version but cannot
downgrade an unknown protocol to v1.

`driver` belongs to the nearest preceding `project` and may appear at most once
for that project. Its protocol must match `v[1-9][0-9]*`, and its package must pass
Go import-path validation. A known directive with a wrong argument count, malformed
protocol, or malformed package is an error in both strict and lax metadata parsing.

### Invocation frame and fields

The driver executable receives an argv frame beginning with `xgo-driver-v1` and one
action:

`Request.Version` is not encoded as a separate option. The first two argv elements
MUST be the case-sensitive `xgo-driver-v1` and `run`/`build`; the decoder derives
`Version == "v1"` from the preamble. The producer MUST start the driver through a
structured process API that accepts an argument slice and MUST NOT invoke it through a
shell.

```text
xgo-driver-v1 run   <common-options> <graph-flag>* <build-flag>* -- <application-arg>*
xgo-driver-v1 build <common-options> <graph-flag>* <build-flag>* --output=<staging> --final-output=<final>
```

The canonical encoder MUST emit options in the following order. Exactly one of
`<selected-source>` and `<replacement>` is present, `<pack>` is an optional complete
group, and each repeatable flag block preserves the order of XGo's normalized slice:

```text
xgo-driver-v1 <action>
  --project-dir=...
  --project-file=...
  --module-root=...
  --driver-package=...
  --selected-path=...
  --selected-version=...
  --origin-main=true|false
  ( --selected-dir=... --selected-gomod=... |
    --replace-path=... --replace-version=... --replace-dir=... --replace-gomod=... )
  --project-ext=...
  --project-full-ext=...
  [ --pack-dir=... --pack-index=... ]
  --declaration-file=...
  --declaration-sha256=...
  --target-modfile=...
  --target-modfile-sha256=...
  --go-command=...
  --graph-work-dir=...
  --go-work=...
  { --graph-flag=... }
  { --build-flag=... }
  ( -- <application-arg>* |
    --output=<staging> --final-output=<final> )
```

`Parse` MUST accept valid protocol options in any order before the run delimiter or
after the build action, preserving semantic compatibility. Placement of singular
options and interleaving among option classes are not part of identity; internal order
within each repeatable flag slice and within application arguments MUST be preserved. A
singular option is present exactly once or at most once according to its
required/optional condition below. `graph-flag` and `build-flag` may repeat as option
names, but an underlying flag name may not repeat. An option splits at its first `=` and
its value may contain later `=` characters. v1 defines no quoting, percent encoding,
response file, or environment substitution. A NUL in any argv element is invalid.

Every option is one `--name=value` argument. The first independent `--` in `run` is the
only protocol delimiter. Empty strings, arguments containing spaces, dash-prefixed
arguments, and a literal later `--` are then passed unchanged as separate elements.
`build` accepts neither a protocol delimiter nor application arguments. The producer
MUST reject an invalid request it constructs before spawn. The consumer independently
MUST reject duplicate, unknown, missing, invalidly combined, or action-inapplicable
fields and exit with a request-validation error before acquisition, build, Engine, or
other side effects. The protocol uses no request file and does not consume stdin.

| Field | Constraint |
| --- | --- |
| `project-dir` / `project-file` | Shared validation requires absolute clean paths and structurally requires the file to be top-level in the directory |
| `module-root` | Shared validation requires an absolute clean path that lexically contains `project-dir` |
| `project-ext` / `project-full-ext` | Non-empty and NUL-free; copied from XGo target resolution; SPX requires `.spx` / `main.spx`, and the project-file basename must be `main.spx` |
| `driver-package` | Valid Go import path inside the `selected-path` module |
| `selected-path` / `selected-version` | MVS logical selection; path is a valid Go module path, the main version is empty, and other modules have a canonical version |
| `origin-main` | Exactly `true` or `false` |
| `selected-dir` / `selected-gomod` | Required as a pair without a replacement; describes selected source only at the structural layer |
| `replace-path` / `replace-version` / `replace-dir` / `replace-gomod` | Required as a complete group with a replacement; then `selected-dir`/`selected-gomod` are forbidden |
| `declaration-file` / `declaration-sha256` | Declaring `gox.mod` or `gop.mod` and its lowercase SHA-256 |
| `target-modfile` / `target-modfile-sha256` | Modfile actually read by XGo while resolving the class graph and its lowercase SHA-256; structurally bound without requiring containment in the project or module root |
| `go-command` / `graph-work-dir` / `go-work` | Go executable, working directory, and workspace used by graph operations; directory/file uses canonical project dir and package uses caller cwd; `go-work=off` disables workspace |
| `graph-flag` | Shared codec recognizes `-mod=mod|readonly|vendor`, `-modfile=<abs>`, and `-overlay=<abs>`; consumers decide whether special policies may execute |
| `build-flag` | Only `-v=true`, `-x=true`, `-work=true`, `-trimpath=true`, and `-buildvcs=false` are supported |
| `pack-dir` / `pack-index` | Optional but paired; directory is `.` or a portable relative slash path, and index is a portable plain file name; SPX additionally rejects `.` |
| `output` / `final-output` | Build-only, absolute, and lexically different; staging is XGo-private and final is the user-visible target |

Option presence and whether its value may be empty are separate rules.
`selected-version` is always present and is empty for a main module. Without a
replacement, both selected-source options are present and all four replacement options
are absent; with a replacement the inverse applies, and a local replacement still emits
an empty `replace-version`. The pack pair may be omitted together. Both build-output
options are present together and both are absent for run. Except for the explicitly
empty version cases, an empty value does not satisfy a required field.

Each digest is exactly 64 lowercase hexadecimal characters. Graph/build flags use a
non-empty `-name=value` form and are unique by name. The shared layer requires only
lexically different spellings for `output` and `final-output`; it does not assert that
they identify different files. SPX launchpack validation rejects staging inside the
pack/asset root but permits XGo's designated transaction elsewhere in `ProjectDir`.
Portable pack paths use `/`; each element excludes
characters below U+0020, backslashes, Windows-invalid characters or reserved device names,
and trailing dots or spaces.

`ResolvedModule` equality includes `Main`, selected path/version/source fields, and
replacement information. A local replacement path uses clean absolute directory
spelling; a versioned replacement retains module path/version but is rejected by
SPX's published policy. `Main=true` requires an empty selected version and no
replacement; a non-main selection requires a non-empty canonical module version. A
selected source without replacement and an effective replacement source both provide
absolute clean `Dir/GoMod` values. A local replacement path uses the same clean absolute
spelling as its replacement directory and has an empty version. The shared codec performs only filesystem-free structural
checks: absolute/clean paths, lexical containment, option groups, import/module
versions, and digest shapes. XGo resolves real paths and pins source provenance
during discovery. The SPX consumer repeats `Lstat`, `EvalSymlinks`, path equality,
regular-file/directory/executable type, containment where applicable to that field,
`SameFile` across reads, and content-digest checks. A containment rule for one field
MUST NOT be indiscriminately applied to every identity path. Passing shared validation
therefore does not assert that a file exists or that a path is not a symlink.

`FileIdentity` is an inseparable `{Path, SHA256}` pair: the path names the file bytes
actually read by the producer, and SHA-256 identifies those exact bytes. `Declaration`
MUST be a direct child of the effective driver-source directory whose basename is `gox.mod`
or `gop.mod`. `TargetModFile` MUST come from the same resolution that produced the
class graph. It may be a project `go.mod`, an explicit `-modfile`, or a `.mod` file in the
module download cache; project/module containment and active-modfile equality MUST NOT
be imposed on it. An identity mismatch terminates the current request and MUST NOT
silently refresh its digest; the caller may only restart discovery and create a new
request. v1 therefore does not add the complete module graph, `go.sum`, or
`go.work.sum` to the wire request. A consumer snapshots additional live graph files
only as required by its own execution. The graph verifier pins only the exact
`TargetModFile`; active-graph rules add its matching sum only when it is also the active
modfile, never merely because a sidecar is adjacent.

Before starting the driver, XGo enforces an argv/environment budget. On Unix, the
executable, every argv/environment string and terminating NUL, plus
`8 * (len(args) + len(env) + 3)` bytes for 64-bit native pointers share a `128 KiB`
limit. On Windows, the executable costs `UTF16Len+1` and every argument costs the
conservative quoting bound `2*UTF16Len+3`; the command line is limited to `30,000` code
units. The environment block, including each item and final terminating NUL, is limited
to `32,767` UTF-16 code units. Either overflow returns `ErrDriverArgvTooLarge`; XGo MUST
NOT truncate arguments or switch to an undefined request file.

### Graph, target, and flag policy

For ordinary directory/file/package targets, XGo snapshots ambient `GOFLAGS`
and `GOWORK` once. It reads and parses `GOFLAGS`, then queries `GOWORK` with
neutral `GOFLAGS=-x=false`. Every supported graph flag is subsequently passed
as a distinct Go-command argv. Subprocess environments pin the same non-empty
no-op and pin `GOWORK` to a canonical go.work path or `off`. This applies to
discovery, driver-package validation, driver build, SPX provenance,
source-bridge build, and launcher build, preventing an ambient flag from
changing the graph at any stage.

Inputs that require classification before rejection have distinct contracts:

| Input | Discovery/classification | After a driver match |
| --- | --- | --- |
| `pkg@version` | Download and inspect the requested version and its class graph in a new temporary module with `GOWORK=off` and `GOFLAGS=-mod=mod`; never reuse caller-graph metadata | Report that v1 does not support `@version`; the probe classifies only and never executes a driver; return `NotHandled` when no driver matches |
| `-mod=vendor` | Only active main/workspace metadata is authoritative; an ordinary target without an external class marker may return `NotHandled`; fail closed before classification when external class metadata cannot be proven from the vendor snapshot | Any driver match reports vendor unsupported; shared codec acceptance exists only to faithfully represent and defensively reject the policy |
| `-overlay` | Use the overlay view only to decide whether target/metadata declares a driver; ordinary targets retain legacy behavior | Since v1 snapshots physical filesystem contents, report overlay unsupported and never pass overlay contents to the driver |

A directory contains exactly one matching project file; a single-file target must be
that unique file. Multi-file targets and patterns containing `...` are rejected only
when they contain a driver-backed project; otherwise legacy handling remains intact.
All graph probes honor context cancellation and clean up their temporary graphs.

### Dispatch state machine

The XGo dispatcher has exactly two legal outcomes:

```text
target -> graph -> class metadata -> driver match
                                |-- NotHandled -> legacy GenGo
                                `-- matched -> validate -> build driver -> invoke
```

The internal decision is tri-state; an error is never another spelling of no match:

| State | Meaning | Result |
| --- | --- | --- |
| `NoMatch` | No target/graph-authorized driver has been proven to exist, or a compatibility probe explicitly allowed here delegates the original input to unchanged legacy handling | Return `NotHandled` |
| `Matched` | Matching project metadata in the target contains any `driver` declaration, including an unknown protocol | Enter post-match gates; every later failure is terminal |
| `Indeterminate` | An authoritative graph/metadata input exists but a read, parse, provenance, boundary, or TOCTOU check cannot complete | Terminal error; never downgrade |

The dispatcher has the following normative phases. Phase boundaries and no-fallback
rules are MUST requirements; when several terminal errors apply, which diagnostic wins
is not part of wire identity:

| Phase | Required behavior |
| --- | --- |
| 0. CLI target | Run the existing target parser exactly once and pass typed `DirProj`/`FilesProj`/`PkgPathProj` to the dispatcher before GenGo or source parsing; a driver does not reinterpret raw argv |
| 1. Policy snapshot | Canonicalize cwd/Go command, snapshot ambient GOFLAGS once, and query/pin GOWORK under neutral GOFLAGS; sampling failure is terminal except for explicitly deferred inputs |
| 2. Target classification | For a directory/file, canonicalize the parent while retaining leaf identity; an unsafe-leaf error may be deferred only to a positive match and a symlink cannot change driver ownership |
| 3. Graph/owner | Anchor directory/file graphs at canonical project dir and package graphs at caller cwd; select the unique deepest effective module root containing the project, reject tied owners as ambiguous, and read the actual target-modfile identity |
| 4. Driver match | Require a matching project file in the target directory and that project's own driver declaration; a driver on another project in the module is not a match |
| 5. Post-match gates | Validate target form, unique project/file identity, provenance, XGo version, protocol, pack/declaration, disabled/recursive guard, and deferred flags; every failure is terminal |
| 6. Driver build | Revalidate the target modfile, validate package provenance, revalidate, build under the same graph, validate the host executable, and revalidate again |
| 7. Handoff | Strictly encode, enforce the argv/environment budget, and perform the final target-modfile check before spawn; the consumer rebinds request identity |
| 8. Supervise/finalize | Wait for the managed driver/Engine chain; cancellation can never become success; commit build/install staging only after success without cancellation |

`FilesProj` classifies files in input order. The whole target may return `NotHandled`
only when every file is `NoMatch`; a terminal error stops immediately, while any
`Matched` result rejects the multi-file target before a driver starts. The final
component of a driver-backed direct-file target remains regular, non-symlink, and
identity-stable and denotes the unique project file. A recursive pattern is one whose
clean form is `...` or ends in `/...`; scanning does not cross nested modules and skips
symlink directories, `vendor`, `testdata`, and dot/underscore-prefixed directories.

`NotHandled` is the only result that permits the legacy path. Once matched, graph,
metadata, version, protocol, driver-build, driver-exit, and asset errors are terminal;
XGo must not retry GenGo. `XGO_DRIVER=off` is also an explicit error, not fallback.
The XGo dispatcher rejects every nested driver dispatch; SPX also rejects a directly
nested SPX driver invocation.

`run` resolves, builds a temporary driver, encodes argv, and launches it with inherited
stdin/stdout/stderr. `build` and `install` resolve all targets before creating staging or
an install directory. The driver must create one non-empty, non-symlink host executable
at the designated staging path; other files in that private directory are not committed
and are removed with it. XGo validates identity and performs the final same-filesystem
rename only at the commit point; failures before that point cannot alter an existing
output. XGo hands off a staging path that is initially absent in a `0700` private
transaction directory beside the final target. The transaction directory can therefore
be inside `ProjectDir` when the final target is there. The driver may write only the
designated staging file, and MUST NOT open, create, remove, rename, or chmod
`final-output`; every
final-target mutation belongs to XGo's commit boundary. `-work=true` retains diagnostics
but does not change commit semantics.

Only the host dispatcher owns signal subscriptions. Driver/Engine supervisors consume
cancellation and its cause. Once cancellation is observed, a child that exits 0 during
shutdown is still not success; Unix preserves a normal code or signal, while Windows
uses a Job Object and represents interrupts as 130.

`cmd/xgodriver` exits with code `2` for argv/protocol parse or live-request validation
failure. Acquisition, graph, build, packaging, and other execution errors use code `1`
when no more specific child status exists. A normal non-zero Engine code is preserved.
On Unix, a signal status is reproduced by re-signalling the wrapper itself, with
`128+signal` only as the fallback when re-signalling fails. Windows has no POSIX signal
status, and a host interrupt returns `130`. `driverprotocol.Encode/Parse/Validate`
return Go errors and define no process exit code. Producer encode, budget, or pre-spawn
identity failures happen before the driver starts and therefore are not
`cmd/xgodriver` code 2. Command errors on stderr have exactly one `xgodriver: ` prefix;
error text is not wire ABI, and callers MUST NOT classify errors by parsing strings.

After a driver match, the following boundaries define the minimum revalidation
sequence. A failure at any boundary is terminal and MUST NOT return to legacy handling:

| Boundary | Invariant that MUST still hold |
| --- | --- |
| After importing resolved metadata | The target modfile and declaration canonical path, type, and SHA-256 still equal their discovery snapshots |
| Around driver-package validation and driver build | Effective module identity, driver-package provenance, target modfile, and graph policy are unchanged |
| Before driver spawn | The target modfile matches again, argv/environment fit the platform budget, and staging remains XGo-private |
| Before any SPX Go query or acquisition | Request declaration, target modfile, module source, and project/pack paths pass live identity validation |
| Around source-bridge and launcher builds | Selection identity and required/optional graph-file snapshots are unchanged, and internal Go commands did not modify original graph files |
| Before Engine spawn or build commit | Verified published assets and payload still match their digests under a cache lease; source assets retain verified regular/non-symlink identity; output came only from the designated staging path |

These are identity boundaries, not a global lock over the source tree. v1 does not
promise to prevent concurrent source edits during an ordinary Go compilation; it fails
closed at the boundaries that define metadata, graph, bundle, and committed output.

### XGo version checks

The declaring module's `xgo` directive and the `driver v1` protocol baseline are
independent and combined by taking the maximum. The current baseline is `1.8.0`.
Source/workspace builds of XGo can expose a standard Go pseudo-version in build info
(for example `v1.2.0-pre.1.0.20260821130422-831eec0b6b4e`); this denotes a development
build and is compared using driver capability `1.8.0`, while diagnostics retain the
original version and capability. A real release prerelease such as `v1.8.0-rc1` is not
treated as development automatically. This rule applies only to XGo capability checks;
SPX published mode still rejects pseudo-versions of the SPX module.

### SPX modes and environment inputs

SPX uses source mode only for the main module, current workspace module, or an
unversioned local replacement. A canonical unreplaced
`github.com/goplus/spx/v3@vX.Y.Z[-prerelease]` uses published mode. A canonical
prerelease may use assets from its own exact tag; pseudo-versions, versioned
replacements, and foreign modules fail closed.

The source bridge's build info must record the effective SPX module as its main module;
the generated launcher must record it as a dependency. For a source/workspace main
module, the Go-generated main build-info version (including a standard pseudo-version)
is diagnostic only and does not participate in identity comparison; module path and
verified graph/source snapshots fix its identity. That workspace dependency's version
in a launcher must be empty or `(devel)`, preventing accidental linkage to versioned
SPX source.

Source-mode runtime precedence is fixed: explicit local runtime manifest -> verified
runtime release/cache (or `SPX_RUNTIME_ASSET_DIR` mirror) -> online acquisition
(skipped offline) -> exact-version source/GOPATH runtime fallback only when the release
is genuinely unavailable. The bridge package is fixed by the effective SPX source,
cannot be replaced by the environment, and is built for the host with
`CGO_ENABLED=1`.

An explicit local runtime manifest MUST completely match the current lock. An
automatically discovered exact-version source/GOPATH manifest binds only runtime
version, host, and actual file contents; it does not invent release provenance. Only a
release-unavailable result or offline cache miss may enter source/GOPATH fallback.
Cancellation, malformed manifests, version/size/digest mismatch, and explicit
local/mirror errors are terminal. With `SPX_RUNTIME_ASSET_DIR`, the manifest MUST come
from the mirror; an existing exact-digest cache hit may supply an asset, while a cache
miss reads the mirror and mirror failure never switches to network or GOPATH. In
contrast, `SPX_DRIVER_ASSET_DIR` requires the manifest and bundle to be actually read
and verified from that explicit mirror; a warm cache cannot hide missing or damaged
mirror content.

Published mode accepts only the combined driver bundle. Programmatic
`launchpack.Config` runtime source roots, runtime manifest paths, runtime asset
directories, and source bridge packages conflict with this mode and are rejected.
Inherited `SPX_RUNTIME_LOCAL_MANIFEST` and `SPX_RUNTIME_ASSET_DIR` values are ignored in
published mode, never participate in selection, and do not become errors merely by
being present or duplicated. Local/GOPATH
bridges likewise never participate in published selection, without requiring errors
for unrelated ambient variables. The only local published-artifact mirror is
`SPX_DRIVER_ASSET_DIR`, which must provide both the exact manifest and host bundle and
pass full verification. A missing or damaged explicit mirror fails without being
hidden by a warm cache or the network.

The environment inputs are:

| Variable | Semantics |
| --- | --- |
| `SPX_RUNTIME_LOCAL_MANIFEST` | Select and strictly verify a source-mode local runtime manifest; failure does not fall back; ignored and excluded from selection in published mode |
| `SPX_RUNTIME_ASSET_DIR` | Select a source-mode runtime-release mirror whose contents must match its manifest; ignored and excluded from selection in published mode |
| `SPX_DRIVER_ASSET_DIR` | Published-mode mirror containing `driver-manifest.json` and the host ZIP; must be an absolute clean path; source mode does not treat it as runtime input |
| `SPX_RUNTIME_CACHE` | Absolute, clean content-addressed cache root |
| `SPX_RUNTIME_OFFLINE` | `1/true/yes/on` forbids network and accepts only a complete verified cache/local hit |
| `GOWORK` / `GOFLAGS` | XGo pins the request to a canonical workspace path/`off`; SPX snapshots that file and uses an equivalent private workspace for its Go subprocesses; all graph/build flags travel in argv with neutral `GOFLAGS=-x=false` |

`ProjectDir`, `AssetDir`, and `SessionDir` are independent roots; user arguments cannot
override the driver's `SessionDir --path`. `.config`, pack indexes, module metadata, and
release manifests are consumed as snapshots and revalidated. “Project read-only” means
SPX-owned driver, scaffold, cache, and build operations may write only XGo's designated
staging file, not other project paths. v1 provides no OS sandbox that prevents Engine or
user application logic from writing files itself.

The SPX graph verifier derives a deterministic selection identity from structured
`go list -m -json all` results and snapshots the presence and contents of the effective
active modfile (default `go.mod` or explicit `-modfile`) and its matching sum,
the requested workspace file and its `<GOWORK>.sum` sidecar, plus the `go.mod/go.sum` reported for every workspace main
module and unversioned local replacement, including a local SPX module. Source-bridge
and launcher builds repeat module selection and file snapshots before and after critical
commands. A selection change, content change, identity change, or appearance/removal
of a required or optional graph file is terminal. Project-driver v1 pins module selection
and graph metadata, not every byte in local module source trees; concurrent source edits
retain the ordinary Go build semantics. When workspace mode is active, SPX creates a
private stable copy, resolves relative `use` and workspace-level local `replace` paths
against the original workspace directory, and directs all of its Go commands to that
copy. The original workspace and sum remain verifier inputs, while Go's mandatory
workspace-sum maintenance is confined to the private copy. Host Go environments used
for graph, provenance, and launcher operations pin host `GOOS/GOARCH`, `CGO_ENABLED=0`,
neutral `GOFLAGS=-x=false`, and the private workspace path or `off`. The non-empty value
overrides persisted GOENV settings. Source-bridge builds additionally remove
inherited `CGO_*` variables and pin `CGO_ENABLED=1`. The Engine process does not inherit
these Go graph/target variables.

The driver package and public `github.com/goplus/spx/v3/x/xgolauncher` package both
MUST match their complete `ResolvedModule` under that graph. Absolute local
replacements use file-identity equality; relative replacements retain and compare the
spelling reported by `go list -m` under the same graph. Build info records SPX as main
for a bridge and as a dependency for a launcher; main/workspace build-info versions and
dirty VCS state are diagnostic only. A published direct run needs no continuing graph
verifier after package origin is established and it no longer executes Go commands.
Source bridges and every launcher build remain verified around critical commands.

Private-graph construction is part of the v1 contract:

- In workspace mode, SPX MUST stably read the original `go.work` and optional
  `<go.work>.sum`, write private copies, and resolve relative `use` and workspace-level
  local `replace` paths against the original workspace directory. Every later Go
  command is pinned to that private `GOWORK`.
- With `GOWORK=off` and `-mod=mod`, SPX MUST first resolve an explicit `-modfile` or the
  active `GOMOD`, stably copy the modfile and matching sum to private
  `graph.mod/graph.sum`, rewrite relative local replacements to absolute paths against
  the active module root (the directory of the neutral-query `GOMOD`, not the directory
  of an external explicit modfile), and replace or append exactly one
  `-modfile=<private graph.mod>`.
- `-mod=readonly` with `GOWORK=off` needs no private writable modfile, while workspace
  mode still performs the private copy above. Both remain subject to the same
  snapshot/verifier contract. SPX MUST NOT rewrite an explicitly selected `-mod=mod`
  to readonly merely to keep the project unchanged.
- Original mod/work files and sums remain verifier inputs. Only a main-module `GoMod`
  reported for the private copy may map back to the request's original path, and only
  when the private path, `Main=true`, module path, and directory all match. No other
  provenance field may be normalized or ignored.
- Private workspace/modfile copies and sums MUST be cleaned on success, failure, and
  cancellation. `-work=true` may retain bridge/launcher/session diagnostics but does not
  make internal graph copies part of the public diagnostics contract.

### Compatibility and protocol evolution

v1 is a closed schema with no implicit minor-version negotiation. A new metadata
`driver vN` and wire preamble `xgo-driver-vN` are both required to add an action; add or
remove a field; change field meaning, cardinality, empty-value rules, digest formulas,
unknown/duplicate handling, or the accepted-frame set; permit a new graph/build flag;
or change run/build process or transaction semantics. Since a v1 consumer rejects
unknown fields, a producer MUST NOT probe a v1 frame with an extra option; capability
negotiation occurs in the metadata protocol value, not argv.

The canonical frame in this section, including the required target-modfile identity,
is the v1 freeze point. Earlier experimental producers or consumers that omitted these
fields are outside the compatibility contract. The codec-containing mod release, XGo
producer, and SPX consumer form one compatible set through a coordinated sequence:
publish mod first, then bump XGo and SPX. Mixed experimental endpoints are unsupported;
after a driver match, the currently supported dispatcher MUST fail closed rather than
guessing identity or entering GenGo.

An implementation fix that does not change the observable contract may remain v1, such
as restoring a missing snapshot revalidation, failing earlier, or refactoring a cache
without changing selection precedence. Decoder tolerance for option order is transport
compatibility only; it does not permit duplicate singular fields, ignored unknown
fields, or non-canonical flag spelling.

Three-repository conformance tests MUST collectively cover the same golden request
shape: the mod codec pins exact canonical argv and round trips, the XGo producer asserts
its output order and budget, and the SPX consumer asserts parsing and live validation.
The suite covers order-independent parsing, missing/duplicate/unknown fields, both
source-provenance branches, the run delimiter, build output, and graph/build flags; it
does not require a copied cross-repository fixture. A protocol change is complete only
after the mod codec, XGo producer, SPX consumer, and this section are updated together
and pass a local-workspace test.

### Cache and lease contract

- A cache key is `namespace + full digest`. Entries with different namespaces or full
  digests MUST NOT be shared; a truncated digest, file name, or interface digest is not
  a substitute.
- A production cache uses cross-process shared/exclusive locking. Publication and
  repair hold the exclusive lock; a shared lease remains held through the final read or
  Engine use. After converting exclusive ownership to a shared lease, the entry MUST be
  revalidated rather than assumed unchanged during conversion.
- An entry is completely written, verified, and synced in a sibling temporary path,
  then published with a same-filesystem rename. Failure or cancellation cannot create a
  valid cache hit. Normal errors SHOULD remove temporary state; an orphan left by a
  process crash is never recognized as an entry.
- Every cache hit revalidates the corresponding manifest, type, size, and SHA-256. A
  future GC obeys leases and cannot delete an in-use component; v1 MAY omit quotas and
  automatic GC.
- Leases coordinate cooperating processes; they are not an OS sandbox against an
  uncooperative same-UID writer. The local trust-root boundary for a published raw
  manifest remains the one defined in the threat model above.

### Published driver manifest and host ZIP

`driver-manifest.json` is limited to `16 MiB` and uses strict JSON: unknown fields,
duplicate keys, trailing values, and wrong types are rejected. The complete schema is
below; array order is part of the v1 contract:

```json
{
  "schema": 1,
  "spx_version": "vX.Y.Z",
  "runtime_version": "R",
  "bundles": [
    {
      "goos": "darwin",
      "goarch": "amd64",
      "name": "spx-driver-darwin-amd64.zip",
      "size": 1,
      "sha256": "<64 lowercase hex>",
      "engine_interface_digest": "<64 lowercase hex>",
      "files": [
        {"name": "gdspxrtR", "mode": 493, "size": 1, "sha256": "<64 lowercase hex>"},
        {"name": "gdspxrtR.pck", "mode": 420, "size": 1, "sha256": "<64 lowercase hex>"},
        {"name": "gdspx-darwin-amd64.dylib", "mode": 493, "size": 1, "sha256": "<64 lowercase hex>"}
      ]
    }
  ]
}
```

The real `bundles` array contains exactly four elements in this order; the one-element
array above demonstrates fields only:

| Order | Target | ZIP | Files, strictly Engine/PCK/bridge |
| --- | --- | --- | --- |
| 1 | `darwin/amd64` | `spx-driver-darwin-amd64.zip` | `gdspxrtR` `0755`; `gdspxrtR.pck` `0644`; `gdspx-darwin-amd64.dylib` `0755` |
| 2 | `darwin/arm64` | `spx-driver-darwin-arm64.zip` | `gdspxrtR` `0755`; `gdspxrtR.pck` `0644`; `gdspx-darwin-arm64.dylib` `0755` |
| 3 | `linux/amd64` | `spx-driver-linux-amd64.zip` | `gdspxrtR` `0755`; `gdspxrtR.pck` `0644`; `gdspx-linux-amd64.so` `0755` |
| 4 | `windows/amd64` | `spx-driver-windows-amd64.zip` | `gdspxrtR.exe` `0755`; `gdspxrtR.pck` `0644`; `gdspx-windows-amd64.dll` `0755` |

Here `R` is `runtime_version` without a leading `v`. Every size is positive and equals
the actual byte count. A runtime or driver archive size declared by a manifest is also
at most `8 GiB`; the consumer MUST reject an oversized manifest before any
mirror/cache/network asset fetch rather than waiting for download, disk write, or ZIP
extraction. Bundle SHA-256 covers the complete ZIP; file SHA-256 covers uncompressed
file bytes. `engine_interface_digest =
SHA256(ASCII("spx-engine-interface/v1") || 0x00 || hexDecode(engine.sha256) ||
hexDecode(pck.sha256))`. `spx_version` equals the graph-selected exact module version,
including its leading `v` (a canonical prerelease is allowed, a pseudo-version is not),
and `runtime_version` equals the SPX runtime lock. The release tag is exactly the
`spx_version` string; driver asset URLs use that SPX Release. No separate
`driver-<spx_version>` Release is created or referenced.

The release packager writes exactly three regular entries in Engine/PCK/bridge order,
using ZIP `Store`, UTC `1980-01-01T00:00:00Z`, the modes above, and portable basenames;
it emits no directory, extra, duplicate, or symlink entry. The same three inputs must
produce identical ZIP bytes. Consumers additionally verify the manifest's exact
archive size/SHA-256 and each file name/mode/size/SHA-256; a file name or interface
digest alone never establishes trust.

### Archive and payload limits

Every limit fails closed before extraction or launcher build; compression, ZIP64, and
manifest declarations cannot bypass it:

| Object | v1 limits | Deterministic parameters |
| --- | --- | --- |
| Runtime/driver ZIP verifier | At most `10,000` entries; `512 MiB` per entry; `4 GiB` total uncompressed; `8 GiB` archive; `200:1` compression ratio. A driver ZIP additionally contains exactly 3 files | The SPX Release driver ZIP uses `Store`, 1980 epoch, canonical mode/order; manifest pins full archive/file digests |
| Canonical project ZIP | At most `10,000` files; `64 MiB` per file; `256 MiB` total input; `512 MiB` archive | UTF-8 slash paths in byte order, Deflate `BestCompression`, 1980 epoch, every entry `0644` |
| Embedded runtime payload ZIP | At most `10,000` entries including the top-level manifest; `512 MiB` per entry; `4 GiB` total; `8 GiB` archive; `1 MiB` payload manifest | Entry-name order, `Store`, 1980 epoch, executables `0755` and others `0644`; full payload and manifest SHA-256 |

The untrusted-archive verifier used for runtime/driver ZIPs and the embedded payload
rejects absolute/`..`/backslash traversal, NUL, invalid UTF-8, duplicates,
Unicode-normalization or case-fold collisions, file-as-parent layouts, overlapping data
ranges, encrypted entries, and symlink/device/special entries. The project ZIP is a
constrained producer rather than a consumer of arbitrary external ZIPs: it applies the
corresponding portable-path, collision, regular non-symlink file, and size rules to its
allowlisted inputs before packaging. Multi-stage project/payload snapshots reject
changes at boundaries that perform double-read or identity checks. The driver-bundle
packager instead binds the bytes read from each opened regular file by exact size and
SHA-256, and later acquisition must match those digests. The complete project ZIP is
embedded with `Store` as `project/project.zip` without rewriting its canonical bytes.

## Validation plan

Local integration must use one workspace so the shared codec, XGo dispatcher, and SPX
driver do not resolve an older module-cache version. Recommended flow (`CODE` is the
common parent directory):

```sh
integ=$(mktemp -d)
(cd "$integ" && GOWORK=off go work init \
  "$CODE/mod" "$CODE/xgo" "$CODE/spx")

(cd "$CODE/mod" && GOWORK="$integ/go.work" \
  go test ./driverprotocol ./modfile ./modload ./xgomod)
(cd "$CODE/xgo" && GOWORK="$integ/go.work" \
  go test ./cmd/internal/projectdriver)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  go test ./internal/driverbundle ./internal/envutil \
    ./internal/xgodriver ./internal/launchpack ./cmd/xgodriver)
```

The integration workspace is read-only for repository metadata; do not run `go work
sync` and commit the resulting `go.mod/go.sum` rewrites. The CLI smoke test builds a
temporary XGo from the same workspace, then runs a real SPX fixture, builds it, and
runs the standalone launcher. A dirty checkout may add VCS settings to Go build info,
but current provenance compares module path/version/replacement and does not reject
dirty state. Local integration should still pass `-buildvcs=false` to remove ambient
variation and make identity results repeatable, rather than adding separate strict VCS
validation. The first run below verifies a cache miss; the second confirms the same
runtime works from the verified cache in offline mode:

```sh
(cd "$CODE/xgo" && GOWORK="$integ/go.work" \
  go build -o "$integ/xgo" ./cmd/xgo)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  SPX_RUNTIME_CACHE="$integ/cache" SPX_RUNTIME_OFFLINE=0 \
  "$integ/xgo" run -buildvcs=false ./test/CI --headless)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  SPX_RUNTIME_CACHE="$integ/cache" SPX_RUNTIME_OFFLINE=1 \
  "$integ/xgo" run -buildvcs=false ./test/CI --headless)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  SPX_RUNTIME_CACHE="$integ/cache" SPX_RUNTIME_OFFLINE=1 \
  "$integ/xgo" build -buildvcs=false -o "$integ/spx-ci" ./test/CI)
"$integ/spx-ci" --headless
```

The same workspace can smoke-test an interactive tutorial: run
`./tutorial/05-Animation`, send `SIGINT` after the Engine starts, then build it to
`$integ/animation` and launch that executable independently. Both run paths should
return 130, clean up their Engine/driver process trees, and leave the tutorial's tracked
file digest unchanged.

After the codec-containing mod release and dependency bumps, relevant standalone
`GOWORK=off go test` commands must also pass in the XGo and SPX repositories so the
three-repository workspace cannot conceal an unclosed dependency.

`xgo run` and the standalone launcher must each print `SPX_CI_TEST_OK`; `xgo build`
must succeed and produce a non-empty host executable. Full acceptance also covers
legacy behavior for ordinary projects, source mode for main/workspace/local replace,
published-bundle cache miss/hit/offline/concurrency/kill-recovery, argv/stdin/signals,
atomic build/install, same-size tampering, and real Darwin/Linux/Windows host artifacts.
Publication is ordered runtime -> build and verify driver assets -> merge those assets
into the Release whose tag is exactly `spx_version`; the module tag must not be
published while either prerequisite is incomplete.

## Appendix: Minimal end-to-end example

The following external `hello-spx` project shows the shortest complete v1 path. The
example uses the concrete versions SPX `v3.2.4` and runtime lock `2.4.4`; in a real
build, both values come independently from the effective graph and the lock, never from
file names.

### 1. Project input

The application directory is:

```text
hello-spx/
|-- go.mod
`-- game/
    |-- main.spx
    `-- assets/index.json
```

The application only marks the class dependency in `go.mod`:

```go
module example.com/hello

go 1.25

require github.com/goplus/spx/v3 v3.2.4 //xgo:class
```

The SPX module's own `gox.mod` (not copied into the application) declares the project and
driver:

```text
project main.spx Game github.com/goplus/spx/v3 math
driver v1 github.com/goplus/spx/v3/cmd/xgodriver
pack assets index.json
```

### 2. The `xgo run` call

The user runs:

```sh
xgo run ./game -- --level 2
```

Here `./game` is a directory target relative to the current working directory; the
directory must contain exactly one discoverable `main.spx`. The first `--` belongs to
the XGo CLI and separates the target from application arguments. `--level` and `2` are
illustrative application arguments, not fixed XGo or SPX v1 options. With no application
arguments, use `xgo run ./game`.

The real argv also contains the declaration digest, Go command, workspace, and
graph/build flags. The following keeps only the fields that illustrate the protocol:

```text
xgo-driver-v1 run
  --project-dir=/work/hello-spx/game
  --project-file=/work/hello-spx/game/main.spx
  --module-root=/work/hello-spx
  --driver-package=github.com/goplus/spx/v3/cmd/xgodriver
  --selected-path=github.com/goplus/spx/v3
  --selected-version=v3.2.4
  --pack-dir=assets
  --pack-index=index.json
  ...
  --
  --level
  2
```

The user-command separator is consumed by XGo. When XGo builds the driver argv, it writes
the protocol-level `--` again. Thus the application arguments in this frame are two
separate elements, `[]string{"--level", "2"}`; every element after the protocol separator
is forwarded to the Engine unchanged. The protocol does not use a request file or stdin.

Because this is the canonical SPX module at an exact version with no replacement, SPX
selects published mode and reads both assets from the same `v3.2.4` SPX Release:

```text
https://github.com/goplus/spx/releases/download/v3.2.4/driver-manifest.json
https://github.com/goplus/spx/releases/download/v3.2.4/spx-driver-darwin-arm64.zip
```

The manifest must declare `spx_version = "v3.2.4"` and `runtime_version = "2.4.4"`.
The host ZIP must contain exactly these three files and then pass the manifest's archive
and file size/SHA-256 checks:

```text
gdspxrt2.4.4
gdspxrt2.4.4.pck
gdspx-darwin-arm64.dylib
```

After verification, the driver materializes the assets into its cache/session and starts
the Engine; the project source directory remains unchanged.

### 3. The `xgo build` call

The user runs:

```sh
xgo build -o hello ./game
```

The protocol action becomes `build`, and XGo allocates private staging:

```text
xgo-driver-v1 build
  ...
  --output=/work/hello-spx/.xgo-driver-output-<random>/hello
  --final-output=/work/hello-spx/hello
```

The driver may write only the `--output` file. SPX generates there a launcher containing the
Engine, PCK, bridge, and canonical project bundle. XGo atomically commits it to
`--final-output` on the same filesystem. The result is one host Mach-O/ELF/PE executable,
not three files that the user must assemble.

### 4. Release asset relationship

The SPX `v3.2.4` GitHub Release may contain:

```text
v3.2.4
|-- driver-manifest.json
|-- spx-driver-darwin-amd64.zip
|-- spx-driver-darwin-arm64.zip
|-- spx-driver-linux-amd64.zip
|-- spx-driver-windows-amd64.zip
|-- SHA256SUMS
`-- other SPX product assets
```

Each driver ZIP is an independently downloadable, cacheable, and verifiable Release
asset, but it is not another `driver-v3.2.4` GitHub Release. The module `v3.2.4`, driver
assets, and download entry point therefore share one publication identity; the canonical
SPX module tag is exposed only after all runtime/driver prerequisites have passed
verification.

If the application uses an unversioned local replacement:

```text
replace github.com/goplus/spx/v3 => /path/to/spx
```

The protocol frame is otherwise unchanged, but the source identity selects source mode:
the driver builds the bridge from the same effective graph. It neither treats the module
cache as published mode nor borrows published driver assets.
