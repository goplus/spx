#!/usr/bin/env python3
"""Publish immutable GitHub releases; runtime and SPX reuse policies differ."""

import json
import os
from pathlib import Path
import subprocess
import sys


def run(*args, cwd=None, capture=False, check=True):
    return subprocess.run(args, cwd=cwd, check=check, text=True,
                          stdout=subprocess.PIPE if capture else None,
                          stderr=subprocess.PIPE if not check else None)


def release_metadata(repository, tag):
    result = run("gh", "release", "view", tag, "--repo", repository,
                 "--json", "isDraft,assets", capture=True, check=False)
    return json.loads(result.stdout) if result.returncode == 0 else None


def validate_targets(env):
    repository = env["REPOSITORY"]
    run("gh", "api", f"repos/{repository}", "--silent")
    targets = [(env["RUNTIME_TAG"], env["RUNTIME_MANIFEST"], "Runtime", True)]
    if env["OPERATION"] == "publish-release":
        targets.append((env["RELEASE_TAG"], "SHA256SUMS", "SPX", False))
    for tag, marker, label, allow_public_reuse in targets:
        metadata = release_metadata(repository, tag)
        if metadata is None:
            result = run("git", "ls-remote", "--exit-code", "--tags",
                         f"https://github.com/{repository}.git", f"refs/tags/{tag}",
                         capture=True, check=False)
            if result.returncode == 0:
                raise ValueError(f"{label} tag {tag} already exists without a resumable release")
            if result.returncode != 2:
                detail = (result.stderr or "").strip()
                raise ValueError(f"Cannot verify {label} tag {tag}: git ls-remote exited {result.returncode}"
                                 + (f": {detail}" if detail else ""))
            continue
        if not allow_public_reuse or metadata["isDraft"]:
            commit = json.loads(run("gh", "api", f"repos/{repository}/commits/{tag}",
                                    capture=True).stdout)["sha"]
            if commit != env["GITHUB_SHA"]:
                raise ValueError(f"{label} tag {tag} points to {commit}, not this commit {env['GITHUB_SHA']}")
        if not metadata["isDraft"] and marker not in {asset["name"] for asset in metadata["assets"]}:
            raise ValueError(f"{label} tag {tag} is an older release without {marker}; bump the declared release version")


def verify_public_release(env, metadata, runtime):
    tag, repository = env["RELEASE_TAG"], env["REPOSITORY"]
    published = Path("published")
    published.mkdir(exist_ok=True)
    run("gh", "release", "download", tag, "--repo", repository, "--dir", str(published))
    run("sha256sum", "-c", "SHA256SUMS", cwd=published)
    local_assets = sorted(path.name for path in Path("dist").iterdir()
                          if path.is_file() and not path.is_symlink())
    remote_assets = sorted(asset["name"] for asset in metadata["assets"])
    if runtime:
        if local_assets != remote_assets:
            raise ValueError(f"Runtime tag {tag} has an unexpected asset set")
        run("go", "run", "./.github/cmd/runtime-manifest", "--lock", "internal/release/runtime.lock.json",
            "--verify-manifest", str(published / env["RUNTIME_MANIFEST"]), "--asset-directory", str(published))
        print(f"[info] Reusing public runtime release for {env['RUNTIME_VERSION']}")
    else:
        if local_assets != remote_assets or Path("dist/SHA256SUMS").read_bytes() != (published / "SHA256SUMS").read_bytes():
            raise ValueError(f"SPX tag {tag} already exists with different assets")
        print(f"[info] Identical SPX release already exists: {tag}")


def publish(env, *, runtime):
    if runtime and env["RUNTIME_STATE"] != "missing":
        raise ValueError(f"Invalid runtime release state: {env['RUNTIME_STATE']}")
    tag, repository = env["RELEASE_TAG"], env["REPOSITORY"]
    metadata = release_metadata(repository, tag)
    if metadata is not None and not metadata["isDraft"]:
        verify_public_release(env, metadata, runtime)
        return

    assets = sorted(str(path) for path in Path("dist").iterdir() if not path.name.startswith("."))
    if not assets:
        raise ValueError("No release assets found in dist")
    if metadata is not None:
        for asset in metadata["assets"]:
            if asset["name"]:
                run("gh", "release", "delete-asset", tag, asset["name"], "--repo", repository, "--yes")
    else:
        if runtime:
            title = f"SPX Runtime {env['RUNTIME_VERSION']}"
            notes = ("--notes", f"Versioned SPX runtime assets. See {env['RUNTIME_MANIFEST']} for checksums.")
        else:
            title, notes = tag, ("--generate-notes",)
        run("gh", "release", "create", tag, "--repo", repository,
            "--target", env["GITHUB_SHA"], "--title", title, *notes, "--draft")
    run("gh", "release", "upload", tag, *assets, "--repo", repository)
    if runtime:
        run("gh", "release", "edit", tag, "--repo", repository, "--draft=false")


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ("validate-targets", "runtime", "spx"):
        print("usage: publish.py {validate-targets|runtime|spx}", file=sys.stderr)
        return 2
    try:
        if sys.argv[1] == "validate-targets":
            validate_targets(os.environ)
        else:
            publish(os.environ, runtime=sys.argv[1] == "runtime")
    except subprocess.CalledProcessError as error:
        print(f"[error] {error}", file=sys.stderr)
        return error.returncode if error.returncode > 0 else 1
    except (KeyError, OSError, ValueError) as error:
        print(f"[error] {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
