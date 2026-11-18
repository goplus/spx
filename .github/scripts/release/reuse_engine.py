#!/usr/bin/env python3
"""Stage verified engine assets without copying the previous runtime pack."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

from download import download_release


def reuse_engine(env):
    with tempfile.TemporaryDirectory(prefix="spx-engine-source-") as temporary:
        source = Path(temporary)
        download_release(env["ENGINE_TAG"], env["REPOSITORY"], source)
        manifest = source / env["RUNTIME_MANIFEST"]
        subprocess.run(("go", "run", "./.github/cmd/runtime-manifest",
                        "--verify-engine-manifest", str(manifest),
                        "--asset-directory", str(source), "--revision", "HEAD"), check=True)
        # The Go verifier binds the manifest to its own immutable lock, compares
        # engine inputs with HEAD, and verifies every engine asset before staging.
        metadata = json.loads(manifest.read_text())
        if env["ENGINE_TAG"] != "runtime-v" + metadata["runtime_version"]:
            raise ValueError("Engine source manifest does not match the selected release tag")
        destination = Path("reused-engine")
        destination.mkdir()
        for asset in metadata["assets"]:
            name = asset["name"]
            if name != "spx-runtime-assets.zip":
                shutil.copyfile(source / name, destination / name)
        shutil.copyfile(manifest, destination / "engine-source-manifest.json")


if __name__ == "__main__":
    try:
        reuse_engine(os.environ)
    except (KeyError, OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"[error] {error}", file=sys.stderr)
        sys.exit(1)
