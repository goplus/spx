#!/usr/bin/env python3
"""Validate release inputs, runtime reuse, and the selected package targets."""

import json
import os
from pathlib import Path
import string
import sys

PLATFORMS = ("web", "macos", "windows", "linux")
NORMALIZE_PLATFORMS = str.maketrans(string.ascii_uppercase, string.ascii_lowercase, string.whitespace)
LOCK = Path("internal/release/runtime.lock.json")


def validate_inputs(env):
    if env["RELEASE_TAG_INPUT"] != env["EXPECTED_RELEASE_TAG"]:
        raise ValueError(f"This commit declares {env['EXPECTED_RELEASE_TAG']}, not {env['RELEASE_TAG_INPUT']}")
    if env["OPERATION_INPUT"] not in ("dry-run", "publish-runtime", "publish-release"):
        raise ValueError(f"Unsupported release operation: {env['OPERATION_INPUT']}")
    if env["OPERATION_INPUT"] != "dry-run" and env["CURRENT_REPOSITORY"] != env["RELEASE_REPOSITORY"]:
        raise ValueError(f"Publishing is locked to {env['RELEASE_REPOSITORY']}, not {env['CURRENT_REPOSITORY']}")


def resolve_platforms(env):
    selected = set()
    if env["OPERATION_INPUT"] != "publish-runtime":
        platforms = env["PLATFORMS_INPUT"].translate(NORMALIZE_PLATFORMS)
        if not platforms:
            raise ValueError("Platforms must not be empty")
        if env["OPERATION_INPUT"] == "publish-release" and platforms != "all":
            raise ValueError("Publishing requires platforms=all")
        items = list(PLATFORMS) if platforms == "all" else platforms.split(",")
        if "" in items:
            raise ValueError("Platforms must not contain empty values")
        for platform in items:
            if platform not in PLATFORMS:
                raise ValueError(f"Unsupported platform: {platform}")
            if platform in selected:
                raise ValueError(f"Duplicate platform: {platform}")
            selected.add(platform)
    with Path(env["GITHUB_OUTPUT"]).open("a") as output:
        for platform in PLATFORMS:
            output.write(f"run_{platform}={str(platform in selected).lower()}\n")


def plan_inputs(env):
    validate_inputs(env)
    resolve_platforms(env)


def enforce_runtime_policy(env, development=False):
    state = env["RUNTIME_STATE"]
    if state == "ready" or (state == "missing" and not development):
        return
    if state not in ("missing", "incompatible"):
        raise ValueError(f"Unknown runtime state: {state or '<empty>'}")
    reason = env["RUNTIME_REASON"]
    title = "Published runtime required" if state == "missing" else "Runtime version bump required"
    print(f"::error title={title}::{reason}")
    print(f"[error] {reason}", file=sys.stderr)
    if state == "missing":
        raise ValueError("Publish the declared runtime before publishing a development npm package.")
    operation = "publishing" if development else "releasing"
    raise ValueError(f"Run make bump-release SPX_VERSION=v4.x.y RUNTIME_VERSION=x.y.z before {operation}.")


def summarize_ancestry(env):
    valid = True
    godot_ref = env["GODOT_REF"]
    if env["RUNTIME_STATE"] == "ready":
        status = f"not required — {env['RUNTIME_REASON']}"
        ref_commit = "not checked (immutable runtime reuse)"
    else:
        statuses = {
            "ancestor": "verified ancestor — source build is eligible for publication",
            "premerge": "pre-merge candidate — dry-run only; publication is blocked",
        }
        valid = env["ANCESTRY_STATUS"] in statuses
        status = statuses.get(env["ANCESTRY_STATUS"], "verification failed — see the ancestry step log")
        ref_commit = env.get("GODOT_REF_COMMIT") or "not resolved"
        if not godot_ref or not env["GODOT_REF_COMMIT"]:
            status = "verification failed — verifier returned incomplete identity outputs"
            valid = False
    if not godot_ref:
        godot_ref = json.loads(LOCK.read_text())["godot"]["ref"]
    rows = (
        ("Operation", env["OPERATION_INPUT"]), ("Locked candidate", env["GODOT_COMMIT"]),
        ("Godot repository", env["GODOT_REPOSITORY"]), ("Canonical ref", godot_ref),
        ("Canonical ref commit", ref_commit),
    )
    summary = ["### Godot source ancestry", "", "| Field | Value |", "| --- | --- |"]
    summary.extend(f"| {name} | `{value}` |" for name, value in rows)
    summary.append(f"| Ancestry policy | {status} |")
    with Path(env["GITHUB_STEP_SUMMARY"]).open("a") as output:
        output.write("\n".join(summary) + "\n")
    return 0 if valid else 1


def validate_dev_target(env):
    expected = json.loads(LOCK.read_text())["release_repository"]
    if env["CURRENT_REPOSITORY"] != expected:
        raise ValueError(f"Development npm publishing is locked to {expected}, not {env['CURRENT_REPOSITORY']}")
    if env["CURRENT_REF"] != "refs/heads/dev":
        raise ValueError(f"Development npm publishing requires refs/heads/dev, not {env['CURRENT_REF']}")
    if env["RELEASE_TAG_INPUT"]:
        raise ValueError("release_tag must be empty for publish-dev-npm")


def main():
    commands = {
        "inputs": plan_inputs,
        "runtime-policy": enforce_runtime_policy, "ancestry-summary": summarize_ancestry,
        "dev-target": validate_dev_target,
        "dev-runtime-policy": lambda env: enforce_runtime_policy(env, development=True),
    }
    command = commands.get(sys.argv[1] if len(sys.argv) > 1 else "")
    if command is None:
        print(f"usage: {sys.argv[0]} {{{'|'.join(commands)}}}", file=sys.stderr)
        return 2
    try:
        return command(os.environ) or 0
    except (KeyError, OSError, ValueError) as error:
        print(f"[error] {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
