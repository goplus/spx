#!/usr/bin/env python3
"""Check, verify, or synchronize the immutable runtime lock snapshot."""

import argparse
import copy
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

import runtime_build_contract as contract


DEFAULT_LOCK_PATH = Path("internal/release/runtime.lock.json")
DEFAULT_SNAPSHOT_DIRECTORY = Path("internal/release/runtime_locks")
EXPECTED_GODOT_REPOSITORY = "https://github.com/goplus/godot.git"
GODOT_CANDIDATE_DEPTHS = (1, 2, 32)
GODOT_TARGET_DEPTHS = (1, 2, 32, 128, 512)
VERIFY_TARGET_REF = "refs/spx-pin/declared"
VERIFY_CANDIDATE_REF = "refs/spx-pin/candidate"


class SnapshotError(ValueError):
    """A user-facing runtime lock snapshot error."""


class GitRunner:
    """Run isolated Git commands without consulting a local Godot clone."""

    def __init__(self, timeout_seconds=60):
        self.timeout_seconds = timeout_seconds

    def run(self, args, cwd=None, check=True):
        environment = os.environ.copy()
        for variable in (
            "GIT_ALTERNATE_OBJECT_DIRECTORIES",
            "GIT_COMMON_DIR",
            "GIT_CONFIG_COUNT",
            "GIT_CONFIG_PARAMETERS",
            "GIT_DIR",
            "GIT_INDEX_FILE",
            "GIT_OBJECT_DIRECTORY",
            "GIT_SHALLOW_FILE",
            "GIT_WORK_TREE",
        ):
            environment.pop(variable, None)
        environment["GIT_TERMINAL_PROMPT"] = "0"
        environment["GIT_CONFIG_GLOBAL"] = os.devnull
        environment["GIT_CONFIG_NOSYSTEM"] = "1"
        command = ["git", *(str(argument) for argument in args)]
        try:
            result = subprocess.run(
                command,
                cwd=cwd,
                env=environment,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                check=False,
                timeout=self.timeout_seconds,
            )
        except subprocess.TimeoutExpired as error:
            raise SnapshotError(
                f"{' '.join(command)} timed out after {self.timeout_seconds} seconds"
            ) from error
        if check and result.returncode != 0:
            detail = result.stderr.strip() or result.stdout.strip() or f"exit {result.returncode}"
            raise SnapshotError(f"{' '.join(command)} failed: {detail}")
        return result


def require_expected_godot_repository(lock):
    repository = lock["godot"]["repository"]
    if repository != EXPECTED_GODOT_REPOSITORY:
        raise SnapshotError(
            f"Godot repository {repository!r} is not the trusted canonical repository "
            f"{EXPECTED_GODOT_REPOSITORY!r}"
        )
    return repository


def remote_ref_candidates(godot_ref, git_runner):
    if godot_ref.startswith("refs/heads/") or godot_ref.startswith("refs/tags/"):
        candidates = [godot_ref]
    elif godot_ref.startswith("refs/"):
        raise SnapshotError(
            f"Godot ref {godot_ref!r} must be a branch or tag, not another refs/* namespace"
        )
    else:
        if godot_ref.startswith("-"):
            raise SnapshotError(f"Godot ref {godot_ref!r} must not begin with '-'")
        candidates = [f"refs/heads/{godot_ref}", f"refs/tags/{godot_ref}"]
    for candidate in candidates:
        git_runner.run(["check-ref-format", candidate])
    return candidates


def resolve_remote_godot_ref(repository, godot_ref, git_runner):
    candidates = remote_ref_candidates(godot_ref, git_runner)
    result = git_runner.run(["ls-remote", "--refs", repository, *candidates])
    matches = []
    candidate_set = set(candidates)
    for line in result.stdout.splitlines():
        fields = line.split("\t", 1)
        if len(fields) == 2 and fields[1] in candidate_set:
            matches.append((fields[1], fields[0]))
    if not matches:
        raise SnapshotError(
            f"Godot ref {godot_ref!r} does not exist as an exact branch or tag in {repository}"
        )
    if len(matches) != 1:
        names = ", ".join(name for name, _ in sorted(matches))
        raise SnapshotError(
            f"Godot ref {godot_ref!r} is ambiguous in {repository}: {names}; use a full refs/heads/* or refs/tags/* ref"
        )
    return matches[0]


def fetch_ref(git_runner, repository_path, depth, refspec):
    git_runner.run(
        [
            "-c",
            "protocol.version=2",
            "fetch",
            "--quiet",
            "--no-tags",
            "--no-recurse-submodules",
            "--filter=blob:none",
            f"--depth={depth}",
            "--",
            "origin",
            refspec,
        ],
        cwd=repository_path,
    )


def git_output(git_runner, repository_path, *args):
    return git_runner.run(list(args), cwd=repository_path).stdout.strip()


def target_ref_is_stable(git_runner, repository_path, advertised_object):
    fetched_object = git_output(git_runner, repository_path, "rev-parse", "--verify", VERIFY_TARGET_REF)
    if fetched_object != advertised_object:
        raise SnapshotError(
            f"Godot ref moved during verification: advertised {advertised_object}, fetched {fetched_object}; retry"
        )
    return git_output(
        git_runner,
        repository_path,
        "rev-parse",
        "--verify",
        f"{VERIFY_TARGET_REF}^{{commit}}",
    )


def is_ancestor_if_known(git_runner, repository_path, godot_commit, target_commit):
    object_result = git_runner.run(
        ["cat-file", "-e", f"{godot_commit}^{{commit}}"],
        cwd=repository_path,
        check=False,
    )
    if object_result.returncode != 0:
        return False
    result = git_runner.run(
        ["merge-base", "--is-ancestor", godot_commit, target_commit],
        cwd=repository_path,
        check=False,
    )
    if result.returncode not in (0, 1):
        detail = result.stderr.strip() or result.stdout.strip() or f"exit {result.returncode}"
        raise SnapshotError(f"git merge-base ancestry check failed: {detail}")
    return result.returncode == 0


def fetch_exact_remote_commit(git_runner, repository_path, godot_commit, depth):
    fetch_ref(
        git_runner,
        repository_path,
        depth,
        f"+{godot_commit}:{VERIFY_CANDIDATE_REF}",
    )
    fetched_commit = git_output(
        git_runner,
        repository_path,
        "rev-parse",
        "--verify",
        f"{VERIFY_CANDIDATE_REF}^{{commit}}",
    )
    if fetched_commit != godot_commit:
        raise SnapshotError(
            f"Godot commit fetch resolved {fetched_commit}, want exact commit {godot_commit}"
        )


def confirmed_premerge_result(
    repository,
    full_ref,
    godot_commit,
    target_commit,
    allow_premerge_candidate,
):
    if not allow_premerge_candidate:
        raise SnapshotError(
            f"Godot commit {godot_commit} exists in {repository} but is not an ancestor of "
            f"{full_ref} ({target_commit}); use --allow-premerge-candidate only for explicit "
            "candidate/dry-run validation, never publication"
        )
    return {
        "status": "premerge",
        "ref": full_ref,
        "ref_commit": target_commit,
    }


def verify_godot_ancestry(
    repository,
    godot_ref,
    godot_commit,
    allow_premerge_candidate=False,
    git_runner=None,
):
    """Prove that commit is reachable from the exact canonical branch or tag."""
    git_runner = git_runner or GitRunner()
    full_ref, advertised_object = resolve_remote_godot_ref(repository, godot_ref, git_runner)

    with tempfile.TemporaryDirectory(prefix="spx-godot-ancestry-") as directory:
        target_repository = Path(directory) / "verify.git"
        git_runner.run(["init", "--bare", "--quiet", target_repository])
        git_runner.run(["remote", "add", "origin", repository], cwd=target_repository)
        target_refspec = f"+{full_ref}:{VERIFY_TARGET_REF}"
        fetch_ref(git_runner, target_repository, GODOT_TARGET_DEPTHS[0], target_refspec)
        target_commit = target_ref_is_stable(
            git_runner,
            target_repository,
            advertised_object,
        )
        if godot_commit == target_commit:
            return {
                "status": "ancestor",
                "ref": full_ref,
                "ref_commit": target_commit,
            }

        # A normal pre-merge candidate is ahead of the declared ref. Fetch the
        # exact candidate from the same remote and prove target -> candidate
        # first; this avoids downloading the declared ref's complete history.
        for depth in GODOT_CANDIDATE_DEPTHS:
            fetch_exact_remote_commit(
                git_runner,
                target_repository,
                godot_commit,
                depth,
            )
            if is_ancestor_if_known(
                git_runner,
                target_repository,
                target_commit,
                godot_commit,
            ):
                return confirmed_premerge_result(
                    repository,
                    full_ref,
                    godot_commit,
                    target_commit,
                    allow_premerge_candidate,
                )

        # Otherwise look for the candidate behind the declared ref. Bounded
        # deepening is intentionally fail-closed: absence from a shallow graph
        # is not proof of non-ancestry.
        for depth in GODOT_TARGET_DEPTHS[1:]:
            fetch_ref(git_runner, target_repository, depth, target_refspec)
            target_commit = target_ref_is_stable(
                git_runner,
                target_repository,
                advertised_object,
            )
            if is_ancestor_if_known(
                git_runner,
                target_repository,
                godot_commit,
                target_commit,
            ):
                return {
                    "status": "ancestor",
                    "ref": full_ref,
                    "ref_commit": target_commit,
                }

    raise SnapshotError(
        f"could not prove whether Godot commit {godot_commit} is an ancestor of "
        f"{full_ref} ({target_commit}) within the bounded remote history; ancestry is unknown"
    )


def verify_locked_godot_ancestry(
    lock_path=DEFAULT_LOCK_PATH,
    allow_premerge_candidate=False,
    ancestry_verifier=verify_godot_ancestry,
):
    lock, _ = load_canonical_lock(lock_path)
    repository = require_expected_godot_repository(lock)
    return ancestry_verifier(
        repository,
        lock["godot"]["ref"],
        lock["godot"]["commit"],
        allow_premerge_candidate=allow_premerge_candidate,
    )


def write_godot_ancestry_outputs(path, result):
    contract.write_github_outputs(
        path,
        {
            "godot_ancestry_status": result["status"],
            "godot_ref": result["ref"],
            "godot_ref_commit": result["ref_commit"],
        },
    )


def report_godot_ancestry(result, godot_commit):
    if result["status"] == "ancestor":
        print(
            f"[ok] Godot commit {godot_commit} is an ancestor of "
            f"{result['ref']} ({result['ref_commit']})"
        )
    else:
        print(
            f"[warning] Godot commit {godot_commit} is a confirmed pre-merge candidate: "
            f"it exists in the canonical repository but is not an ancestor of "
            f"{result['ref']} ({result['ref_commit']}); publication is blocked"
        )


def canonical_runtime_lock(lock):
    """Encode a validated lock in the same stable field order as the Go type."""
    canonical = {
        "schema": lock["schema"],
        "runtime_version": lock["runtime_version"],
        "runtime_abi": lock["runtime_abi"],
        "release_repository": lock["release_repository"],
        "manifest": lock["manifest"],
        "required_assets": lock["required_assets"],
        "godot": {
            "repository": lock["godot"]["repository"],
            "ref": lock["godot"]["ref"],
            "commit": lock["godot"]["commit"],
            "version": lock["godot"]["version"],
        },
        "module": {
            "path": lock["module"]["path"],
        },
        "toolchain": {
            field: lock["toolchain"][field] for field in contract.TOOLCHAIN_FIELDS
        },
    }
    encoded = json.dumps(canonical, ensure_ascii=False, indent=2) + "\n"
    # encoding/json escapes these characters even when emitting UTF-8.
    return (
        encoded.replace("&", r"\u0026")
        .replace("<", r"\u003c")
        .replace(">", r"\u003e")
        .replace("\u2028", r"\u2028")
        .replace("\u2029", r"\u2029")
        .encode("utf-8")
    )


def load_canonical_lock(lock_path):
    lock_path = Path(lock_path)
    lock = contract.load_runtime_lock(lock_path)
    canonical = canonical_runtime_lock(lock)
    if lock_path.read_bytes() != canonical:
        raise SnapshotError(
            f"runtime lock {lock_path} is not canonical two-space JSON; format it before synchronizing its snapshot"
        )
    return lock, canonical


def current_snapshot_path(lock, snapshot_directory):
    return Path(snapshot_directory) / f"{lock['runtime_version']}.json"


def check_snapshot(lock_path=DEFAULT_LOCK_PATH, snapshot_directory=DEFAULT_SNAPSHOT_DIRECTORY):
    lock, canonical = load_canonical_lock(lock_path)
    snapshot_path = current_snapshot_path(lock, snapshot_directory)
    try:
        snapshot = snapshot_path.read_bytes()
    except FileNotFoundError as error:
        raise SnapshotError(
            f"current runtime lock snapshot is missing: {snapshot_path}; run this command with --sync"
        ) from error
    if snapshot != canonical:
        raise SnapshotError(
            f"current runtime lock snapshot differs from {lock_path}: {snapshot_path}; "
            "if this runtime is still unpublished, rerun with --sync --allow-unpublished-update"
        )
    return snapshot_path


def write_file_atomically(path, data):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(
        dir=path.parent,
        prefix=f".{path.name}.",
    )
    temporary_path = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        os.chmod(temporary_path, 0o644)
        os.replace(temporary_path, path)
    finally:
        try:
            temporary_path.unlink()
        except FileNotFoundError:
            pass


def write_files_transactionally(updates):
    """Atomically replace each file and roll earlier replacements back on error."""
    originals = []
    for path, _ in updates:
        path = Path(path)
        try:
            originals.append((path, path.read_bytes()))
        except FileNotFoundError:
            originals.append((path, None))

    applied = 0
    try:
        for path, data in updates:
            write_file_atomically(path, data)
            applied += 1
    except OSError as error:
        rollback_errors = []
        for path, original in reversed(originals[:applied]):
            try:
                if original is None:
                    path.unlink(missing_ok=True)
                else:
                    write_file_atomically(path, original)
            except OSError as rollback_error:
                rollback_errors.append(f"{path}: {rollback_error}")
        if rollback_errors:
            raise SnapshotError(
                f"runtime lock pin failed ({error}) and rollback was incomplete: {', '.join(rollback_errors)}"
            ) from error
        raise


def sync_snapshot(
    lock_path=DEFAULT_LOCK_PATH,
    snapshot_directory=DEFAULT_SNAPSHOT_DIRECTORY,
    allow_unpublished_update=False,
):
    lock, canonical = load_canonical_lock(lock_path)
    snapshot_path = current_snapshot_path(lock, snapshot_directory)
    try:
        existing = snapshot_path.read_bytes()
    except FileNotFoundError:
        existing = None
    if existing == canonical:
        return snapshot_path, False
    if existing is not None and not allow_unpublished_update:
        raise SnapshotError(
            f"refusing to replace existing snapshot {snapshot_path}; published snapshots are immutable. "
            "Use --allow-unpublished-update only after confirming runtime-v"
            f"{lock['runtime_version']} is not public"
        )
    write_file_atomically(snapshot_path, canonical)
    return snapshot_path, True


def pin_godot(
    lock_path,
    snapshot_directory,
    godot_commit,
    godot_ref,
    allow_unpublished_update=False,
    allow_premerge_candidate=False,
    ancestry_verifier=verify_godot_ancestry,
):
    lock_path = Path(lock_path)
    lock, _ = load_canonical_lock(lock_path)
    updated_lock = copy.deepcopy(lock)
    updated_lock["godot"]["commit"] = godot_commit
    updated_lock["godot"]["ref"] = godot_ref
    contract.validate_runtime_lock(updated_lock)
    repository = require_expected_godot_repository(updated_lock)
    canonical = canonical_runtime_lock(updated_lock)
    snapshot_path = current_snapshot_path(updated_lock, snapshot_directory)

    try:
        existing_snapshot = snapshot_path.read_bytes()
    except FileNotFoundError:
        existing_snapshot = None
    if existing_snapshot not in (None, canonical) and not allow_unpublished_update:
        raise SnapshotError(
            f"refusing to replace existing snapshot {snapshot_path}; published snapshots are immutable. "
            "Use --allow-unpublished-update only after confirming runtime-v"
            f"{updated_lock['runtime_version']} is not public"
        )

    ancestry = ancestry_verifier(
        repository,
        godot_ref,
        godot_commit,
        allow_premerge_candidate=allow_premerge_candidate,
    )

    updates = []
    if existing_snapshot != canonical:
        updates.append((snapshot_path, canonical))
    if lock_path.read_bytes() != canonical:
        updates.append((lock_path, canonical))
    write_files_transactionally(updates)
    return snapshot_path, bool(updates), ancestry


def build_parser():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true", help="verify that the current snapshot exactly matches the lock")
    mode.add_argument("--sync", action="store_true", help="create or explicitly refresh the current snapshot")
    mode.add_argument(
        "--pin-godot",
        action="store_true",
        help="update the current lock's Godot commit/ref and synchronize its snapshot as one operation",
    )
    mode.add_argument(
        "--verify-godot-ancestry",
        action="store_true",
        help="verify that the locked Godot commit is reachable from its exact canonical branch or tag",
    )
    parser.add_argument("--lock", default=DEFAULT_LOCK_PATH, type=Path, help="path to runtime.lock.json")
    parser.add_argument(
        "--snapshot-directory",
        default=DEFAULT_SNAPSHOT_DIRECTORY,
        type=Path,
        help="directory containing versioned runtime lock snapshots",
    )
    parser.add_argument(
        "--allow-unpublished-update",
        action="store_true",
        help="allow --sync or --pin-godot to replace the current snapshot after confirming it is unpublished",
    )
    parser.add_argument(
        "--allow-premerge-candidate",
        action="store_true",
        help=(
            "allow an exact commit proven to descend from the declared ref only for an explicit candidate/dry-run; "
            "never use this for publication"
        ),
    )
    parser.add_argument(
        "--github-output",
        type=Path,
        help="append structured Godot ancestry outputs for --verify-godot-ancestry",
    )
    parser.add_argument("--godot-commit", help="full lowercase Godot source commit for --pin-godot")
    parser.add_argument("--godot-ref", help="durable Godot branch or tag for --pin-godot")
    return parser


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.allow_unpublished_update and not (args.sync or args.pin_godot):
        parser.error("--allow-unpublished-update requires --sync or --pin-godot")
    if args.allow_premerge_candidate and not (
        args.verify_godot_ancestry or args.pin_godot
    ):
        parser.error(
            "--allow-premerge-candidate requires --verify-godot-ancestry or --pin-godot"
        )
    if args.github_output and not args.verify_godot_ancestry:
        parser.error("--github-output requires --verify-godot-ancestry")
    if args.pin_godot:
        if not args.godot_commit or not args.godot_ref:
            parser.error("--pin-godot requires --godot-commit and --godot-ref")
    elif args.godot_commit or args.godot_ref:
        parser.error("--godot-commit and --godot-ref require --pin-godot")
    try:
        if args.check:
            snapshot_path = check_snapshot(args.lock, args.snapshot_directory)
            print(f"[ok] Current runtime lock snapshot matches: {snapshot_path}")
        elif args.sync:
            snapshot_path, changed = sync_snapshot(
                args.lock,
                args.snapshot_directory,
                args.allow_unpublished_update,
            )
            action = "Updated" if changed else "Already current"
            print(f"[ok] {action}: {snapshot_path}")
        elif args.verify_godot_ancestry:
            ancestry = verify_locked_godot_ancestry(
                args.lock,
                args.allow_premerge_candidate,
            )
            godot_commit = contract.load_runtime_lock(args.lock)["godot"]["commit"]
            report_godot_ancestry(ancestry, godot_commit)
            if args.github_output:
                write_godot_ancestry_outputs(args.github_output, ancestry)
        else:
            snapshot_path, changed, ancestry = pin_godot(
                args.lock,
                args.snapshot_directory,
                args.godot_commit,
                args.godot_ref,
                args.allow_unpublished_update,
                args.allow_premerge_candidate,
            )
            action = "Pinned Godot source and synchronized" if changed else "Godot source already pinned"
            print(f"[ok] {action}: {snapshot_path}")
            report_godot_ancestry(ancestry, args.godot_commit)
    except (contract.ContractError, SnapshotError, OSError) as error:
        print(f"[error] Invalid runtime lock snapshot: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
