#!/usr/bin/env python3
"""Verify the npm publication source, select its version, and publish its bundle."""

import argparse
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import stat
import subprocess
import sys
import zipfile


def capture(*command):
    return subprocess.run(command, check=True, text=True, stdout=subprocess.PIPE).stdout.rstrip("\n")


def verify_source(env):
    current_sha = capture("git", "rev-parse", "HEAD")
    if current_sha != env["TARGET"]:
        raise ValueError(f"Checked out {current_sha} instead of publication target {env['TARGET']}")
    if not env["RELEASE_VERSION"]:
        tags = capture("git", "tag", "--points-at", "HEAD", "--list", "v4.*").splitlines()
        if tags:
            raise ValueError(f"Refusing to publish development npm from exact release tag {tags[0]}")


def resolve_version(env):
    version = env["RELEASE_VERSION"] or capture("go", "run", "./.github/cmd/npm-version", "HEAD")
    version = version.removeprefix("v")
    print(f"[info] SPX version: {version}")
    return version


def prepare_package(version):
    package_dir = Path(".npm/spx")
    if package_dir.parent.is_symlink():
        raise ValueError("npm workspace must not be a symbolic link: .npm")
    with zipfile.ZipFile("spx_web.zip") as archive:
        members = archive.infolist()
        if not members:
            raise ValueError("Web bundle is empty")
        for member in members:
            path = PurePosixPath(member.filename)
            if path.is_absolute() or ".." in path.parts or "\\" in member.filename or stat.S_ISLNK(member.external_attr >> 16):
                raise ValueError(f"Unsafe web bundle entry: {member.filename}")
        # The caller cannot supply a cleanup path. Only this generated package
        # directory is replaced; sibling npm archives remain untouched.
        if package_dir.is_symlink() or package_dir.is_file():
            package_dir.unlink()
        elif package_dir.exists():
            shutil.rmtree(package_dir)
        package_dir.mkdir(parents=True)
        archive.extractall(package_dir)
        for member in members:
            mode = (member.external_attr >> 16) & 0o777
            if mode and not member.is_dir():
                (package_dir / member.filename).chmod(mode)

    metadata = {
        "name": "@xgo-pkgs/spx",
        "version": version,
        "description": "SPX web runtime bundle.",
        "license": "Apache-2.0",
        "repository": {"type": "git", "url": "git+https://github.com/goplus/spx.git"},
        "homepage": "https://github.com/goplus/spx#readme",
        "bugs": {"url": "https://github.com/goplus/spx/issues"},
        "files": ["**/*"],
    }
    (package_dir / "package.json").write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
    (package_dir / "README.md").write_text(
        "# @xgo-pkgs/spx\n\nSPX web runtime bundle for browser-based runners.\n\n"
        "This package is generated from `spx_web.zip` in the SPX release pipeline.\n",
        encoding="utf-8",
    )


def publish_package(env):
    version = resolve_version(env)
    dist_tag = "dev"
    if env["RELEASE_VERSION"]:
        dist_tag = "next" if "-" in version else "latest"
    prepare_package(version)
    package_directory = Path(".npm/packages")
    package_directory.mkdir(parents=True, exist_ok=True)
    packed = json.loads(capture("npm", "pack", ".npm/spx", "--json", "--pack-destination", str(package_directory)))
    if not isinstance(packed, list) or len(packed) != 1 or not isinstance(packed[0], dict):
        raise ValueError("npm pack must describe exactly one package")
    filename, local_integrity = packed[0].get("filename"), packed[0].get("integrity")
    if not isinstance(filename, str) or not filename or Path(filename).name != filename or filename in (".", ".."):
        raise ValueError("npm pack returned an invalid package filename")
    if not isinstance(local_integrity, str) or not local_integrity:
        raise ValueError("npm pack returned no package integrity")

    remote = subprocess.run(
        ("npm", "view", f"@xgo-pkgs/spx@{version}", "dist.integrity"),
        text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
    )
    if remote.returncode == 0:
        remote_integrity = remote.stdout.rstrip("\n")
        if remote_integrity != local_integrity:
            raise ValueError(f"Existing npm package integrity {remote_integrity} does not match {local_integrity}")
        tags = json.loads(capture("npm", "view", "@xgo-pkgs/spx", "dist-tags", "--json"))
        if not isinstance(tags, dict):
            raise ValueError("npm dist-tags must be an object")
        remote_dist_tag = tags.get(dist_tag) or ""
        if remote_dist_tag != version:
            raise ValueError(f"Existing npm package is not selected by dist-tag {dist_tag} (found {remote_dist_tag})")
        print(f"[info] Identical @xgo-pkgs/spx@{version} already exists")
        return
    subprocess.run(("npm", "publish", str(package_directory / filename), "--access", "public", "--tag", dist_tag), check=True)


def main():
    commands = {"verify-source": verify_source, "publish": publish_package}
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=commands)
    args = parser.parse_args()
    try:
        commands[args.command](os.environ)
    except subprocess.CalledProcessError as error:
        print(f"[error] {error}", file=sys.stderr)
        return error.returncode if error.returncode > 0 else 1
    except (KeyError, OSError, ValueError, zipfile.BadZipFile) as error:
        print(f"[error] {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
