#!/usr/bin/env python3

"""Generate reusable TwitterColorEmoji-SVGinOT subsets."""

from __future__ import annotations

import argparse
import pathlib
import subprocess
import sys
import urllib.request


DEFAULT_EMOJI_VERSION = "15.1"
DEFAULT_PRESET = "heart-only"
UNICODE_SEQUENCES_URL = "https://www.unicode.org/Public/emoji/{version}/emoji-sequences.txt"
PRESET_CODEPOINTS = {
    # Keep the shipped fallback tiny while preserving the colored heart sequence.
    "heart-only": [0x2764, 0xFE0F],
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Subset an emoji font with either a preset or explicit Unicode list."
    )
    parser.add_argument("--source", required=True, help="Path to the source TTF font.")
    parser.add_argument("--output", required=True, help="Path to the subset output TTF.")
    parser.add_argument(
        "--preset",
        default=DEFAULT_PRESET,
        choices=["heart-only", "basic-emoji"],
        help=f"Named subset preset to use (default: {DEFAULT_PRESET}).",
    )
    parser.add_argument(
        "--unicodes",
        help="Comma-separated Unicode list, for example: U+2764,U+FE0F,U+1F600",
    )
    parser.add_argument(
        "--emoji-version",
        default=DEFAULT_EMOJI_VERSION,
        help=(
            "Unicode emoji data version used by the basic-emoji preset "
            f"(default: {DEFAULT_EMOJI_VERSION})."
        ),
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


def parse_unicode_spec(spec: str) -> list[int]:
    codepoints: set[int] = set()
    for raw_token in spec.split(","):
        token = raw_token.strip().upper()
        if not token:
            continue
        if token.startswith("U+"):
            token = token[2:]
        if ".." in token:
            start, end = (int(part, 16) for part in token.split("..", 1))
            codepoints.update(range(start, end + 1))
            continue
        if "-" in token:
            start, end = (int(part, 16) for part in token.split("-", 1))
            codepoints.update(range(start, end + 1))
            continue
        codepoints.add(int(token, 16))
    if not codepoints:
        raise RuntimeError("no codepoints parsed from --unicodes")
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


def resolve_codepoints(args: argparse.Namespace) -> tuple[str, list[int]]:
    if args.unicodes:
        return "custom unicode list", parse_unicode_spec(args.unicodes)
    if args.preset == "basic-emoji":
        return (
            f"Unicode emoji {args.emoji_version} Basic_Emoji",
            fetch_basic_emoji_codepoints(args.emoji_version),
        )
    codepoints = PRESET_CODEPOINTS.get(args.preset)
    if codepoints is None:
        raise RuntimeError(f"unknown preset: {args.preset}")
    return args.preset, sorted(codepoints)


def main() -> int:
    args = parse_args()
    source = pathlib.Path(args.source)
    output = pathlib.Path(args.output)
    subset_name, codepoints = resolve_codepoints(args)
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
    print(f"Subsetting {source.name} with {subset_name} ({len(codepoints)} codepoints).")
    subprocess.run(cmd, check=True)
    print(f"Wrote subset font to {output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
