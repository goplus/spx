#!/usr/bin/env python3
"""Import a Godot project and export its TileMap, decorators and preview."""

import argparse
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys


def copy_addon(source: Path, project: Path) -> None:
    destination = project / "addons" / "spx_tilemap_exporter"
    if source == destination.resolve():
        return
    if source in destination.resolve().parents:
        raise ValueError("The target project cannot be inside the source addon")
    # Update addon files without removing unrelated files from the target project.
    shutil.copytree(source, destination, dirs_exist_ok=True,
                    ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))


def run_godot(godot: str, project: Path, arguments: list) -> None:
    command = [godot, "--path", str(project), *arguments]
    print("Running: " + shlex.join(command), flush=True)
    subprocess.run(command, cwd=project, check=True)


def main() -> int:
    source = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--godot", default=os.environ.get("GODOT_PATH"),
                        metavar="PATH", help="Godot editor executable (default: GODOT_PATH)")
    parser.add_argument("--project", type=Path, default=source.parent.parent,
                        metavar="PATH", help="Target Godot project (default: the project containing this addon)")
    parser.add_argument("--copy", action="store_true",
                        help="Copy this addon into the target project before exporting")
    parser.add_argument("--scene", default="res://main.tscn", metavar="PATH",
                        help="Scene to export; res:// is optional (default: res://main.tscn)")
    parser.add_argument("--no-preview", action="store_true",
                        help="Export data without rendering a preview or opening a window")
    args = parser.parse_args()

    project = args.project.expanduser().resolve()
    if not (project / "project.godot").is_file():
        parser.error("No project.godot in %s; use --project PATH" % project)
    if not args.godot:
        parser.error("Specify --godot PATH or set GODOT_PATH to a Godot editor executable")
    executable = shutil.which(os.path.expanduser(args.godot))
    if not executable:
        parser.error("Godot executable not found or not executable: " + args.godot)
    # Resolve before changing the child's working directory.
    godot = str(Path(executable).resolve())

    try:
        if args.copy:
            copy_addon(source, project)
        script = "addons/spx_tilemap_exporter/export_cli.gd"
        if not (project / script).is_file():
            parser.error("Addon not installed in %s; use --copy" % project)
        run_godot(godot, project, ["--headless", "--import"])
        arguments = ["--script", script, "--", "--scene", args.scene]
        if args.no_preview:
            arguments.insert(0, "--headless")
            arguments.append("--no-preview")
        run_godot(godot, project, arguments)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print("Export failed: " + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
