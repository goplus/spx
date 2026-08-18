# Contributing to SPX

Thank you for your interest in improving SPX. This guide covers the most
important points for reporting bugs, proposing changes, and contributing pull
requests.

For development details, see the [SPX development documentation](docs/en/dev/index.md),
especially the [architecture](docs/en/dev/engine/architecture.md),
[build commands](docs/en/dev/engine/cmd_make.md), and
[binding code generation](docs/en/dev/engine/code_generator.md) guides.

<!--
Automated contribution policy:

An AI agent that opens an issue or pull request for someone else must identify
its involvement. Prefix the title with 🤖 and add a notice like this to the
description:
>
> `Automation notice: An AI agent prepared this submission at the direction of [person or team] for [purpose].`
>
Maintainers may close automated submissions that omit this notice. Repeated
nondisclosure may result in contribution access being restricted.
-->

## Table of contents

- [Reporting bugs](#reporting-bugs)
- [Proposing features or improvements](#proposing-features-or-improvements)
- [Setting up a development environment](#setting-up-a-development-environment)
- [Contributing pull requests](#contributing-pull-requests)
- [Testing your changes](#testing-your-changes)
- [Working across SPX and Godot](#working-across-spx-and-godot)
- [Communicating with maintainers](#communicating-with-maintainers)

## Reporting bugs

Report bugs in the [SPX issue tracker](https://github.com/goplus/spx/issues).
Search existing issues first to avoid duplicates. Include enough information
for another contributor to reproduce the problem:

- the SPX version or commit;
- the operating system and target platform;
- the Go and XGo versions;
- the runtime path used, such as interpreted, native, editor, or Web;
- the Web mode (`normal`, `worker`, `minigame`, or `miniprogram`), when relevant;
- the expected and actual behavior, including complete error output; and
- a minimal reproduction project or a link to a small repository.

Remove generated directories, caches, binaries, and unrelated assets from a
reproduction. Confirm the issue against the latest SPX release and, when
practical, the current default branch. If the problem is a regression, state
the last known working version and the first version that fails.

Security vulnerabilities should not be filed as public issues. Use the
repository's **Security** tab to report them privately.

## Proposing features or improvements

Open an [issue](https://github.com/goplus/spx/issues/new) before investing in a
large feature or architectural change. Explain the user problem, the intended
behavior, alternatives you considered, and any compatibility impact. A small
example showing how the proposed API would be used is especially helpful.

Early discussion helps establish whether the feature belongs in the SPX Go
API, the SPX Godot module, the Godot fork, or a platform-specific bridge. It
also reduces the risk of implementing an approach that cannot be maintained
across all supported runtimes.

## Setting up a development environment

Install Git, Make, Go, and the XGo CLI. Keep `$GOPATH/bin` on `PATH`. Windows
contributors should use Git Bash and install `mingw-w64`.

Fork the repository, clone your fork, and add the canonical repository as an
upstream remote:

```sh
git clone https://github.com/YOUR_ACCOUNT/spx.git
cd spx
git remote add upstream https://github.com/goplus/spx.git
```

Prepare a development environment with the published host runtime:

```sh
make setup
make doctor
make list-demos
make run DEMO_INDEX=1
```

Run `make help` for common commands and `make help-advanced` for lower-level
commands. The root `Makefile` is the supported entry point for normal local
development.

Changes to the engine, the SPX Godot module, native bindings, or Web bridge may
require the [goplus/godot](https://github.com/goplus/godot) source tree. Point
`GODOT_SRC` at that checkout and build the smallest affected target:

```sh
GODOT_SRC=/absolute/path/to/godot make dev MODE=normal
```

Use a focused target such as `make build-editor`, `make build-desktop`, or
`make build-web MODE=normal` when a complete development build is unnecessary.

## Contributing pull requests

Keep each pull request focused on one bug or feature. Before opening it:

- rebase your branch on the current upstream branch and avoid unrelated merge
  commits;
- add or update tests for behavior changes;
- update user or developer documentation when behavior, APIs, or workflows
  change;
- regenerate checked-in output when its source declarations or templates
  change; and
- run the checks appropriate for the affected code and inspect the complete
  diff, including generated files.

In the pull request description, explain the problem, the chosen approach, and
how the change was tested. Include platform and Web mode details when relevant.
Use a GitHub closing keyword such as `Fixes #1234` when the pull request should
close an issue.

Draft pull requests are welcome for early feedback, but mark a pull request as
ready only when the implementation, tests, and documentation are complete.

### Keep commits reviewable

Each commit should leave the repository in a coherent state. Fold follow-up
typo, formatting, and build-fix commits into the commit that introduced the
problem before review is complete.

SPX uses concise, English commit subjects based on the conventional commit
style already used in the repository. Keep the subject around 72 characters
when possible and use a relevant type and optional scope, for example:

```text
fix(audio): clean up completed playback IDs
feat(runtime): add condition event trigger
docs(engine): explain Web worker synchronization
```

Use the commit body to explain why a non-obvious change is needed and note
important compatibility or design decisions.

### Document changes

Public API changes must include clear API documentation and an example where it
improves understanding. Update developer documentation for changes to the
engine boundary, build system, generated bindings, platform behavior, or
release flow.

Keep corresponding files under `docs/en` and `docs/zh` structurally aligned.
Commands, identifiers, paths, and code examples should remain equivalent in
both languages.

### Do not edit generated files directly

Files marked as generated, including files with `.gen.` in their names, must be
updated through their source declarations, generator code, or templates. Run:

```sh
make generate
```

Review every generated change. For binding changes, compile and exercise each
affected native or Web target; successful generation alone does not establish
runtime compatibility.

## Testing your changes

Add focused tests close to the code they cover. Bug fixes should include a test
that fails before the fix. New features should cover normal behavior and
expected failure cases where applicable.

For Go changes, start with the affected package and then run the repository
checks:

```sh
make format
make generate
git diff --check
go test $(go list ./... | grep -v /internal/webffi)
(cd cmd/ispx && go test ./...)
```

The `internal/webffi` package is excluded from the host test command because it
is built for Web targets. Changes to build orchestration should also run:

```sh
go test ./internal/cmd/buildctl/...
```

Compile and run the smallest representative target for runtime changes:

```sh
make run DEMO_INDEX=1
make runnative DEMO_INDEX=1
make runweb DEMO_INDEX=1
```

Choose only the commands relevant to the change, but test every affected
platform boundary. Web changes must use the same mode during setup, build,
export, and execution. Platform-specific changes that cannot be tested locally
must be called out explicitly in the pull request.

GitHub Actions runs generation checks, Go tests, cross-platform builds, and the
appropriate published-runtime or current-module integration path. All required
checks must pass before merge.

## Working across SPX and Godot

SPX spans two repositories:

- [goplus/spx](https://github.com/goplus/spx) owns the Go runtime, CLI, build
  orchestration, generated bindings, and the external module under
  `godot_modules/spx`;
- [goplus/godot](https://github.com/goplus/godot) owns the coupled Godot engine
  fork and its engine-side integration.

When a feature needs changes in both repositories, open linked pull requests
and state which commit or branch was used for integration testing. Keep
`GODOT_SRC` pointed at the intended Godot checkout and do not copy the external
SPX module into the Godot tree.

Do not change runtime pins or release metadata merely to make local testing
work. Those files define published, cross-repository provenance and should only
change as part of an intentional runtime or release update.

## Communicating with maintainers

Use the [issue tracker](https://github.com/goplus/spx/issues) for reproducible
bugs, feature proposals, and discussions tied to a specific change. Use the
discussion on an existing issue or pull request when one already covers the
topic, and keep technical decisions recorded there so future contributors can
find them.

Be respectful, patient, and specific. Reviews may request changes for API
stability, cross-platform behavior, generated-code consistency, or maintenance
cost even when an implementation works on one platform.

Thanks for contributing to SPX.
