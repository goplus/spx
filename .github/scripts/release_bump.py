#!/usr/bin/env python3
"""Advance the paired SPX/runtime release declaration without rewriting history."""

import argparse
import copy
import json
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import quote

import runtime_build_contract as contract
import runtime_lock_snapshot as lock_snapshot


DEFAULT_CURRENT_SPX_VERSION_PATH = Path("internal/release/current_spx_version.go")
DEFAULT_RELEASE_META_PATH = Path("internal/release/release_meta.go")
SEMVER_PATTERN = re.compile(
    r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
    r"(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$"
)
CURRENT_SPX_VERSION_PATTERN = re.compile(
    r'(?m)^const currentSPXVersion = "(v[^"]+)"$'
)
MAPPING_HEADER = "var historicalSPXRuntimeMappings = []spxRuntimeMapping{\n"
MAPPING_ENTRY_PATTERN = re.compile(
    r'\t\{spxVersion: "(v[^"]+)", runtimeVersion: "([^"]+)"\},\n',
)


class ReleaseBumpError(ValueError):
    """A user-facing release declaration error."""


class GitHubClient:
    """Read release/tag state through the authenticated GitHub CLI."""

    def get(self, endpoint, allow_missing=False):
        try:
            result = subprocess.run(
                ["gh", "api", "--hostname", "github.com", "--method", "GET", endpoint],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                check=False,
                timeout=30,
            )
        except FileNotFoundError as error:
            raise ReleaseBumpError(
                "GitHub CLI 'gh' is required to verify release/tag immutability"
            ) from error
        except subprocess.TimeoutExpired as error:
            raise ReleaseBumpError(
                f"GitHub API request timed out: {endpoint}"
            ) from error
        if result.returncode != 0:
            detail = result.stderr.strip() or result.stdout.strip() or f"exit {result.returncode}"
            if allow_missing and "HTTP 404" in detail:
                return None
            raise ReleaseBumpError(f"GitHub API request failed for {endpoint}: {detail}")
        try:
            return json.loads(result.stdout)
        except json.JSONDecodeError as error:
            raise ReleaseBumpError(
                f"GitHub API returned invalid JSON for {endpoint}: {error}"
            ) from error


def github_release_endpoint(repository, tag):
    return f"/repos/{repository}/releases/tags/{quote(tag, safe='')}"


def github_tag_endpoint(repository, tag):
    return f"/repos/{repository}/git/ref/tags/{quote(tag, safe='')}"


def verify_release_boundary(
    repository,
    current_spx_version,
    current_runtime_version,
    new_spx_version,
    new_runtime_version,
    github_client=None,
):
    """Require published current releases and completely unused target tags."""
    github_client = github_client or GitHubClient()
    current_tags = (
        current_spx_version,
        f"runtime-v{current_runtime_version}",
    )
    target_tags = (
        new_spx_version,
        f"runtime-v{new_runtime_version}",
    )
    for tag in current_tags:
        release = github_client.get(github_release_endpoint(repository, tag), allow_missing=True)
        if release is None or release.get("draft") is True:
            raise ReleaseBumpError(
                f"current release {repository}@{tag} is not public; do not archive it as history"
            )
        if release.get("tag_name") != tag:
            raise ReleaseBumpError(
                f"GitHub release lookup for {tag} returned tag {release.get('tag_name')!r}"
            )
        reference = github_client.get(github_tag_endpoint(repository, tag), allow_missing=True)
        if reference is None or reference.get("ref") != f"refs/tags/{tag}":
            raise ReleaseBumpError(
                f"current release tag {repository}@{tag} has no exact Git reference"
            )
    for tag in target_tags:
        if github_client.get(github_release_endpoint(repository, tag), allow_missing=True) is not None:
            raise ReleaseBumpError(
                f"target release already exists and will not be overwritten: {repository}@{tag}"
            )
        if github_client.get(github_tag_endpoint(repository, tag), allow_missing=True) is not None:
            raise ReleaseBumpError(
                f"target Git tag already exists and will not be reused: {repository}@{tag}"
            )


def validate_bumped_release(lock_path, snapshot_directory):
    """Validate the updated snapshot and Go release catalog before commit."""
    lock_snapshot.check_snapshot(lock_path, snapshot_directory)
    try:
        result = subprocess.run(
            ["go", "test", "./internal/release"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
            timeout=120,
        )
    except FileNotFoundError as error:
        raise ReleaseBumpError("Go is required to validate the updated release catalog") from error
    except subprocess.TimeoutExpired as error:
        raise ReleaseBumpError("release catalog tests timed out after 120 seconds") from error
    if result.returncode != 0:
        detail = result.stdout.strip() or f"exit {result.returncode}"
        raise ReleaseBumpError(f"updated release catalog tests failed:\n{detail}")


def parse_semver(value, label, require_spx_major=False):
    raw = value[1:] if require_spx_major and value.startswith("v") else value
    if require_spx_major and not value.startswith("v"):
        raise ReleaseBumpError(f"{label} {value!r} must start with 'v'")
    match = SEMVER_PATTERN.fullmatch(raw)
    if match is None:
        raise ReleaseBumpError(f"{label} {value!r} is not canonical SemVer")
    major, minor, patch = (int(match.group(index)) for index in range(1, 4))
    if require_spx_major and major != 3:
        raise ReleaseBumpError(f"{label} {value!r} must use SPX major version v3")
    prerelease = match.group(4)
    if prerelease is not None:
        for identifier in prerelease.split("."):
            if identifier.isdigit() and len(identifier) > 1 and identifier.startswith("0"):
                raise ReleaseBumpError(
                    f"{label} {value!r} has a non-canonical numeric prerelease identifier"
                )
    return (major, minor, patch, prerelease)


def compare_semver(left, right):
    for left_part, right_part in zip(left[:3], right[:3]):
        if left_part != right_part:
            return -1 if left_part < right_part else 1
    left_pre, right_pre = left[3], right[3]
    if left_pre is None or right_pre is None:
        if left_pre == right_pre:
            return 0
        return 1 if left_pre is None else -1
    left_parts = left_pre.split(".")
    right_parts = right_pre.split(".")
    for left_part, right_part in zip(left_parts, right_parts):
        if left_part == right_part:
            continue
        left_numeric = left_part.isdigit()
        right_numeric = right_part.isdigit()
        if left_numeric and right_numeric:
            return -1 if int(left_part) < int(right_part) else 1
        if left_numeric != right_numeric:
            return -1 if left_numeric else 1
        return -1 if left_part < right_part else 1
    if len(left_parts) == len(right_parts):
        return 0
    return -1 if len(left_parts) < len(right_parts) else 1


def require_newer_version(current, requested, label, require_spx_major=False):
    current_semver = parse_semver(current, f"current {label}", require_spx_major)
    requested_semver = parse_semver(requested, f"new {label}", require_spx_major)
    if compare_semver(requested_semver, current_semver) <= 0:
        raise ReleaseBumpError(
            f"new {label} {requested!r} must be newer than current {current!r}"
        )


def require_regular_file(path):
    path = Path(path)
    if path.is_symlink() or not path.is_file():
        raise ReleaseBumpError(f"managed release file must be a regular file: {path}")


def read_current_spx_version(path):
    source = Path(path).read_text(encoding="utf-8")
    matches = CURRENT_SPX_VERSION_PATTERN.findall(source)
    if len(matches) != 1:
        raise ReleaseBumpError(
            f"expected exactly one currentSPXVersion declaration in {path}, found {len(matches)}"
        )
    return matches[0], source


def update_current_spx_version(source, current_version, new_version):
    old = f'const currentSPXVersion = "{current_version}"'
    new = f'const currentSPXVersion = "{new_version}"'
    if source.count(old) != 1:
        raise ReleaseBumpError("current SPX version declaration changed while preparing bump")
    return source.replace(old, new, 1)


def archive_current_mapping(source, current_spx_version, current_runtime_version, new_spx_version):
    if source.count(MAPPING_HEADER) != 1:
        raise ReleaseBumpError(
            "release metadata must contain exactly one historicalSPXRuntimeMappings declaration"
        )
    body_start = source.index(MAPPING_HEADER) + len(MAPPING_HEADER)
    body_end = source.find("}\n", body_start)
    if body_end < 0:
        raise ReleaseBumpError("historical SPX/runtime mapping list is not terminated")
    body = source[body_start:body_end]
    matches = list(MAPPING_ENTRY_PATTERN.finditer(body))
    if "".join(match.group(0) for match in matches) != body:
        raise ReleaseBumpError(
            "historical SPX/runtime mappings must use the canonical one-entry-per-line form"
        )
    mappings = [(match.group(1), match.group(2)) for match in matches]
    if any(spx_version == new_spx_version for spx_version, _ in mappings):
        raise ReleaseBumpError(f"SPX release {new_spx_version!r} is already historical")
    current_mapping = (current_spx_version, current_runtime_version)
    if current_mapping in mappings:
        raise ReleaseBumpError(
            f"current mapping {current_spx_version} -> {current_runtime_version} is already historical"
        )
    if any(spx_version == current_spx_version for spx_version, _ in mappings):
        raise ReleaseBumpError(
            f"current SPX release {current_spx_version!r} conflicts with historical mappings"
        )
    entry = (
        f'\t{{spxVersion: "{current_spx_version}", '
        f'runtimeVersion: "{current_runtime_version}"}},\n'
    )
    return source[:body_end] + entry + source[body_end:]


def bump_release(
    spx_version,
    runtime_version,
    runtime_abi=None,
    lock_path=lock_snapshot.DEFAULT_LOCK_PATH,
    snapshot_directory=lock_snapshot.DEFAULT_SNAPSHOT_DIRECTORY,
    current_spx_version_path=DEFAULT_CURRENT_SPX_VERSION_PATH,
    release_meta_path=DEFAULT_RELEASE_META_PATH,
    release_guard=verify_release_boundary,
    post_write_validator=validate_bumped_release,
):
    lock_path = Path(lock_path)
    snapshot_directory = Path(snapshot_directory)
    current_spx_version_path = Path(current_spx_version_path)
    release_meta_path = Path(release_meta_path)
    for path in (lock_path, current_spx_version_path, release_meta_path):
        require_regular_file(path)
    if snapshot_directory.is_symlink() or not snapshot_directory.is_dir():
        raise ReleaseBumpError(
            f"runtime lock snapshot directory must be a real directory: {snapshot_directory}"
        )

    lock, _ = lock_snapshot.load_canonical_lock(lock_path)
    lock_snapshot.check_snapshot(lock_path, snapshot_directory)
    current_spx_version, current_source = read_current_spx_version(
        current_spx_version_path
    )
    require_newer_version(
        current_spx_version,
        spx_version,
        "SPX version",
        require_spx_major=True,
    )
    current_runtime_version = lock["runtime_version"]
    require_newer_version(
        current_runtime_version,
        runtime_version,
        "runtime version",
    )
    if runtime_abi is not None:
        if runtime_abi <= lock["runtime_abi"]:
            raise ReleaseBumpError(
                f"new runtime ABI {runtime_abi} must be greater than current {lock['runtime_abi']}"
            )

    target_snapshot = snapshot_directory / f"{runtime_version}.json"
    if target_snapshot.exists() or target_snapshot.is_symlink():
        raise ReleaseBumpError(
            f"refusing to replace existing runtime lock snapshot: {target_snapshot}"
        )

    release_guard(
        lock["release_repository"],
        current_spx_version,
        current_runtime_version,
        spx_version,
        runtime_version,
    )

    release_meta_source = release_meta_path.read_text(encoding="utf-8")
    updated_meta = archive_current_mapping(
        release_meta_source,
        current_spx_version,
        lock["runtime_version"],
        spx_version,
    )
    updated_current = update_current_spx_version(
        current_source,
        current_spx_version,
        spx_version,
    )
    updated_lock = copy.deepcopy(lock)
    updated_lock["runtime_version"] = runtime_version
    if runtime_abi is not None:
        updated_lock["runtime_abi"] = runtime_abi
    contract.validate_runtime_lock(updated_lock)
    canonical_lock = lock_snapshot.canonical_runtime_lock(updated_lock)
    updates = [
        (release_meta_path, updated_meta.encode("utf-8")),
        (current_spx_version_path, updated_current.encode("utf-8")),
        (lock_path, canonical_lock),
        (target_snapshot, canonical_lock),
    ]

    lock_snapshot.write_files_transactionally(
        updates,
        validate=lambda: post_write_validator(lock_path, snapshot_directory),
    )
    return {
        "previous_spx_version": current_spx_version,
        "spx_version": spx_version,
        "previous_runtime_version": lock["runtime_version"],
        "runtime_version": runtime_version,
        "runtime_abi": updated_lock["runtime_abi"],
        "snapshot": target_snapshot,
    }


def build_parser():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("spx_version", metavar="SPX_VERSION", help="new SPX tag, for example v3.3.0")
    parser.add_argument(
        "runtime_version",
        metavar="RUNTIME_VERSION",
        help="new runtime version without the runtime-v prefix, for example 2.5.0",
    )
    parser.add_argument(
        "--runtime-abi",
        type=int,
        help="new runtime ABI; omitted keeps the current ABI",
    )
    parser.add_argument("--lock", default=lock_snapshot.DEFAULT_LOCK_PATH, type=Path)
    parser.add_argument(
        "--snapshot-directory",
        default=lock_snapshot.DEFAULT_SNAPSHOT_DIRECTORY,
        type=Path,
    )
    parser.add_argument(
        "--current-spx-version-file",
        default=DEFAULT_CURRENT_SPX_VERSION_PATH,
        type=Path,
    )
    parser.add_argument(
        "--release-meta",
        default=DEFAULT_RELEASE_META_PATH,
        type=Path,
    )
    return parser


def main(argv=None):
    args = build_parser().parse_args(argv)
    try:
        result = bump_release(
            args.spx_version,
            args.runtime_version,
            args.runtime_abi,
            args.lock,
            args.snapshot_directory,
            args.current_spx_version_file,
            args.release_meta,
        )
    except (contract.ContractError, lock_snapshot.SnapshotError, ReleaseBumpError, OSError) as error:
        print(f"[error] Release bump failed: {error}", file=sys.stderr)
        return 1
    print(
        f"[ok] SPX {result['previous_spx_version']} -> {result['spx_version']}; "
        f"runtime {result['previous_runtime_version']} -> {result['runtime_version']}; "
        f"ABI {result['runtime_abi']}"
    )
    print(f"[ok] Created immutable runtime lock snapshot: {result['snapshot']}")
    print("[ok] Validated current snapshot and Go release catalog")
    print(
        "[next] If Godot also changes, pin the new unpublished snapshot; "
        "then review the diff and run release dry-run."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
