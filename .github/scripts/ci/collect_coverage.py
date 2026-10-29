#!/usr/bin/env python3
"""Collect Go profiles from every required coverage producer for one upload."""

import argparse
from pathlib import Path


def collect_profiles(directory):
    profiles = []
    for source in ("source", "macos", "windows"):
        reports = [path.as_posix() for path in (directory / f"coverage-{source}").rglob("*.out")
                   if path.is_file()]
        if not reports:
            raise SystemExit(f"No coverage profiles found for {source}")
        profiles.extend(reports)
    return sorted(profiles)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--github-output", type=Path, required=True)
    args = parser.parse_args()
    # Do not publish a partial result if any producer is missing.
    profiles = collect_profiles(args.directory)
    with args.github_output.open("a", encoding="utf-8") as output:
        output.write(f"coverage={','.join(profiles)}\n")


if __name__ == "__main__":
    main()
