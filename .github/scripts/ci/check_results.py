#!/usr/bin/env python3
"""Validate the required jobs for the runtime path selected by CI."""

import json
import os
import sys


def require_job_result(needs, job, expected):
    actual = needs.get(job, {}).get("result", "missing")
    if actual != expected:
        raise ValueError(f"{job} result is {actual}, want {expected}")


def check_results(needs):
    for job in ("check-source", "build-go", "resolve-runtime"):
        require_job_result(needs, job, "success")

    runtime = needs["resolve-runtime"]["outputs"]
    state = runtime["runtime_state"]
    if state == "ready":
        require_job_result(needs, "test-godot-module", "skipped")
        require_job_result(needs, "build-web", "success")
        message = "Published runtime verified; Web normal bundle built"
    elif state in ("missing", "incompatible"):
        require_job_result(needs, "test-godot-module", "success")
        require_job_result(needs, "build-web", "skipped")
        message = f"Published runtime is {state}; current SPX module integration passed"
    else:
        raise ValueError(f"Unknown runtime state: {state or '<empty>'}")

    reason = runtime["runtime_reason"]
    print(f"[ok] {message}")
    print(f"[info] Runtime resolution: {reason}")


if __name__ == "__main__":
    try:
        check_results(json.loads(os.environ["NEEDS_JSON"]))
    except KeyError as error:
        print(f"[error] Missing required input: {error.args[0]}", file=sys.stderr)
        sys.exit(1)
    except ValueError as error:
        print(f"[error] {error}", file=sys.stderr)
        sys.exit(1)
