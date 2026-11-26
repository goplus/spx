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
        module_result, message = "skipped", "Published runtime verified"
    elif state in ("missing", "incompatible"):
        module_result, message = "success", f"Published runtime is {state}; current SPX module integration passed"
    else:
        raise ValueError(f"Unknown runtime state: {state or '<empty>'}")

    require_job_result(needs, "test-godot-module", module_result)
    require_job_result(needs, "build-web", "success")
    reason = runtime["runtime_reason"]
    print(f"[ok] {message}; Web normal bundle built")
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
