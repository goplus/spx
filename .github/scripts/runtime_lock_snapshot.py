#!/usr/bin/env python3
"""Check or synchronize the immutable snapshot for the current runtime lock."""

import argparse
import copy
import json
import os
import sys
import tempfile
from pathlib import Path

import runtime_build_contract as contract


DEFAULT_LOCK_PATH = Path("internal/release/runtime.lock.json")
DEFAULT_SNAPSHOT_DIRECTORY = Path("internal/release/runtime_locks")


class SnapshotError(ValueError):
    """A user-facing runtime lock snapshot error."""


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
):
    lock_path = Path(lock_path)
    lock, _ = load_canonical_lock(lock_path)
    updated_lock = copy.deepcopy(lock)
    updated_lock["godot"]["commit"] = godot_commit
    updated_lock["godot"]["ref"] = godot_ref
    contract.validate_runtime_lock(updated_lock)
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

    updates = []
    if existing_snapshot != canonical:
        updates.append((snapshot_path, canonical))
    if lock_path.read_bytes() != canonical:
        updates.append((lock_path, canonical))
    write_files_transactionally(updates)
    return snapshot_path, bool(updates)


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
    parser.add_argument("--godot-commit", help="full lowercase Godot source commit for --pin-godot")
    parser.add_argument("--godot-ref", help="durable Godot branch or tag for --pin-godot")
    return parser


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.allow_unpublished_update and args.check:
        parser.error("--allow-unpublished-update requires --sync or --pin-godot")
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
        else:
            snapshot_path, changed = pin_godot(
                args.lock,
                args.snapshot_directory,
                args.godot_commit,
                args.godot_ref,
                args.allow_unpublished_update,
            )
            action = "Pinned Godot source and synchronized" if changed else "Godot source already pinned"
            print(f"[ok] {action}: {snapshot_path}")
    except (contract.ContractError, SnapshotError, OSError) as error:
        print(f"[error] Invalid runtime lock snapshot: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
