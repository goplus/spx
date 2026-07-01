#!/usr/bin/env python3

"""
Generate a TwitterColorEmoji-SVGinOT subset from the Unicode Basic_Emoji set.
"""

from __future__ import annotations

import argparse
import pathlib
import subprocess
import sys
import urllib.request


DEFAULT_EMOJI_VERSION = "15.1"
UNICODE_SEQUENCES_URL = "https://www.unicode.org/Public/emoji/{version}/emoji-sequences.txt"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Subset an emoji font to the Unicode Basic_Emoji set."
    )
    parser.add_argument("--source", required=True, help="Path to the source TTF font.")
    parser.add_argument("--output", required=True, help="Path to the subset output TTF.")
    parser.add_argument(
        "--emoji-version",
        default=DEFAULT_EMOJI_VERSION,
        help=f"Unicode emoji data version to use (default: {DEFAULT_EMOJI_VERSION}).",
    )
    parser.add_argument(
        "--pyftsubset",
        default="pyftsubset",
        help="Path to the pyftsubset executable.",
    )
    return parser.parse_args()


def fetch_basic_emoji_codepoints(version: str) -> list[int]:
    url = UNICODE_SEQUENCES_URL.format(version=version)
    with urllib.request.urlopen(url) as resp:
        data = resp.read().decode("utf-8")

    codepoints: set[int] = set()
    for raw_line in data.splitlines():
        line = raw_line.split("#", 1)[0].strip()
        if not line or ";" not in line:
            continue
        left, right = (part.strip() for part in line.split(";", 1))
        if right.split()[0] != "Basic_Emoji":
            continue
        for token in left.split():
            if ".." in token:
                start, end = (int(part, 16) for part in token.split("..", 1))
                codepoints.update(range(start, end + 1))
            else:
                codepoints.add(int(token, 16))
    if not codepoints:
        raise RuntimeError(f"no Basic_Emoji data found for Unicode emoji {version}")
    return sorted(codepoints)


def collapse_codepoints(codepoints: list[int]) -> str:
    ranges: list[str] = []
    start = prev = codepoints[0]
    for cp in codepoints[1:]:
        if cp == prev + 1:
            prev = cp
            continue
        ranges.append(format_range(start, prev))
        start = prev = cp
    ranges.append(format_range(start, prev))
    return ",".join(ranges)


def format_range(start: int, end: int) -> str:
    if start == end:
        return f"U+{start:04X}"
    return f"U+{start:04X}-{end:04X}"


def main() -> int:
    args = parse_args()
    source = pathlib.Path(args.source)
    output = pathlib.Path(args.output)
    codepoints = fetch_basic_emoji_codepoints(args.emoji_version)
    unicode_spec = collapse_codepoints(codepoints)

    cmd = [
        args.pyftsubset,
        str(source),
        f"--output-file={output}",
        f"--unicodes={unicode_spec}",
        "--ignore-missing-unicodes",
        "--retain-gids",
        "--passthrough-tables",
    ]
    print(
        f"Subsetting {source.name} with Unicode emoji {args.emoji_version} Basic_Emoji "
        f"({len(codepoints)} codepoints)."
    )
    subprocess.run(cmd, check=True)
    print(f"Wrote subset font to {output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
