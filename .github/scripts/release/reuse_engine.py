#!/usr/bin/env python3
"""Stage verified engine assets without copying the previous runtime pack."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

from download import download_release


def reuse_engine(env):
    tag = env["ENGINE_TAG"]
    repository = env["REPOSITORY"]
    manifest_name = env["RUNTIME_MANIFEST"]
    with tempfile.TemporaryDirectory(prefix="spx-engine-source-") as temporary:
        source = Path(temporary)
        download_release(tag, repository, source, manifest_name)
        manifest = source / manifest_name
        metadata = json.loads(manifest.read_text())
        if tag != "runtime-v" + metadata["runtime_version"]:
            raise ValueError("Engine source manifest does not match the selected release tag")

        engine_names = [asset["name"] for asset in metadata["assets"] if asset["name"] != "spx-runtime-assets.zip"]
        # The Go verifier enforces the complete locked inventory and checksums
        # below. Before download, only allow literal ZIP names as CLI patterns.
        if not engine_names or any(
            not isinstance(name, str) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*\.zip", name)
            for name in engine_names
        ):
            raise ValueError("Engine source manifest must contain literal engine ZIP names")
        download_release(tag, repository, source, *engine_names)
        subprocess.run(("go", "run", "./.github/cmd/runtime-manifest",
                        "--verify-engine-manifest", str(manifest),
                        "--asset-directory", str(source), "--revision", "HEAD"), check=True)

        destination = Path("reused-engine")
        destination.mkdir()
        for name in engine_names:
            shutil.copyfile(source / name, destination / name)
        shutil.copyfile(manifest, destination / "engine-source-manifest.json")


if __name__ == "__main__":
    try:
        reuse_engine(os.environ)
    except (KeyError, OSError, TypeError, ValueError, subprocess.CalledProcessError) as error:
        print(f"[error] {error}", file=sys.stderr)
        sys.exit(1)
