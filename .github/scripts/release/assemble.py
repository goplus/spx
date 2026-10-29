#!/usr/bin/env python3
"""Assemble exact runtime/product asset sets; Go owns runtime manifest semantics."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

LOCK = Path("internal/release/runtime.lock.json")
MANIFEST_COMMAND = ("go", "run", "./.github/cmd/runtime-manifest")
PRODUCTS = {
    "WEB": ("spx_web.zip", "spx_web_worker.zip", "spx_web_minigame.zip", "spx_web_miniprogram.zip"),
    "MACOS": ("spx-standalone-darwin-x64.zip", "spx-standalone-darwin-arm64.zip"),
    "WINDOWS": ("spx-standalone-windows-x64.zip",),
    "LINUX": ("spx-standalone-linux-x64.zip",),
}


def run(*arguments, cwd=None):
    subprocess.run(arguments, cwd=cwd, check=True)


def file_digest(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def collect_artifacts(directory):
    files = {}
    for path in sorted(directory.rglob("*")):
        if not path.is_file() or path.is_symlink():
            continue
        if path.name in files:
            raise ValueError(f"Duplicate asset name {path.name}: {files[path.name]} and {path}")
        if path.stat().st_size == 0:
            raise ValueError(f"Empty asset: {path}")
        files[path.name] = path
    return files


def copy_assets(files, names, directory, label):
    for name in names:
        if name not in files:
            raise ValueError(f"Missing {label}: {name}")
        shutil.copyfile(files.pop(name), directory / name)


def verify_runtime(directory, manifest):
    run(*MANIFEST_COMMAND, "--lock", str(LOCK), "--verify-manifest", str(directory / manifest),
        "--asset-directory", str(directory))


def generate_runtime_manifest(directory, names, env):
    manifest = directory / env["RUNTIME_MANIFEST"]
    checksums = directory / "SHA256SUMS"
    arguments = [*MANIFEST_COMMAND, "--lock", str(LOCK), "--revision", "HEAD", "--output", str(manifest),
                 "--checksums", str(checksums)]
    for name in names:
        arguments.extend(("--asset", f"{name}={directory / name}"))
    run(*arguments)
    entries = checksums.read_text().splitlines()
    entries.append(f"{file_digest(manifest)}  {manifest.name}")
    checksums.write_text("\n".join(sorted(entries, key=lambda line: line[66:])) + "\n")


def require_runtime_files(directory, expected):
    actual = {path.name for path in directory.iterdir() if path.is_file() and not path.is_symlink()}
    unexpected = actual - expected
    if unexpected:
        raise ValueError(f"Unexpected runtime release asset: {', '.join(sorted(unexpected))}")
    missing = expected - actual
    if missing:
        raise ValueError(f"Runtime release asset is missing: {', '.join(sorted(missing))}")


def assemble(env):
    state = env["RUNTIME_STATE"]
    if state not in ("ready", "missing"):
        raise ValueError(f"Invalid runtime release state: {state}")
    artifacts, runtime, product = Path("artifacts"), Path("dist/runtime"), Path("dist/product")
    for directory in (artifacts, runtime, product):
        directory.mkdir(parents=True, exist_ok=True)
    files = collect_artifacts(artifacts)
    if not files and state == "missing":
        raise ValueError("No build artifacts were downloaded")
    required_assets = json.loads(LOCK.read_text())["required_assets"]
    manifest = env["RUNTIME_MANIFEST"]
    if state == "ready":
        run("gh", "release", "download", env["RUNTIME_TAG"], "--repo", env["REPOSITORY"], "--dir", str(runtime))
        run("sha256sum", "-c", "SHA256SUMS", cwd=runtime)
        verify_runtime(runtime, manifest)
    else:
        copy_assets(files, required_assets, runtime, "locked runtime asset")

    selected = [name for platform, names in PRODUCTS.items() if env[f"RUN_{platform}"] == "true" for name in names]
    copy_assets(files, selected, product, "selected SPX product asset")
    if files:
        raise ValueError(f"Unexpected build artifact: {', '.join(sorted(files))}")
    if state == "missing":
        generate_runtime_manifest(runtime, required_assets, env)
        verify_runtime(runtime, manifest)
    require_runtime_files(runtime, {*required_assets, manifest, "SHA256SUMS"})
    if selected:
        entries = [f"{file_digest(product / name)}  {name}\n" for name in sorted(selected)]
        (product / "SHA256SUMS").write_text("".join(entries))

    # Publish outputs only after the complete artifact set has passed validation.
    with Path(env["GITHUB_OUTPUT"]).open("a") as output:
        output.write(f"product_count={len(selected)}\n")
    for label, directory in (("Runtime", runtime), ("SPX product", product)):
        print(f"[info] {label} release assets:")
        print("\n".join(sorted(path.name for path in directory.iterdir() if path.is_file())))


if __name__ == "__main__":
    try:
        assemble(os.environ)
    except (KeyError, OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"[error] {error}", file=sys.stderr)
        sys.exit(1)
