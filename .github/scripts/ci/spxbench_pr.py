#!/usr/bin/env python3
"""Validate a PR's CI artifact and report its spxbench result on the PR head."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys

REPOSITORY = "goplus/spx"
CI_PATH = ".github/workflows/ci.yml"
STATUS_CONTEXT = "spxbench"
CANDIDATE_NAME = re.compile(r"spxbench-pr-([1-9][0-9]*)-([0-9a-f]{40})-([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)")
PR_RUN_NAME = re.compile(r"CI for PR #([1-9][0-9]*) into dev")


def github_api(path, payload=None, paginate=False):
    args = ["gh", "api", f"repos/{REPOSITORY}/{path}"]
    if paginate:
        args += ["--paginate", "--slurp"]
    if payload is not None:
        args += ["--method", "POST", "--input", "-"]
        payload = json.dumps(payload)
    result = subprocess.run(args, input=payload, capture_output=True, text=True, check=True)
    return json.loads(result.stdout)


def github_items(path, key):
    pages = github_api(path, paginate=True)
    return [item for page in pages for item in page[key]]


def workflow_run(*statuses):
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    run = event["workflow_run"]
    if (os.environ["GITHUB_REPOSITORY"] != REPOSITORY
            or event["repository"]["full_name"] != REPOSITORY
            or run["repository"]["full_name"] != REPOSITORY
            or run["event"] != "pull_request" or run["path"] != CI_PATH
            or run["status"] not in statuses
            or not re.fullmatch(r"[0-9a-f]{40}", run["head_sha"])):
        raise ValueError("Expected a canonical PR CI run with status " + " or ".join(statuses))
    return run


def matches_head_repository(run, repository):
    # GitHub may omit a deleted fork from a PR or a workflow run.
    return (repository or {}).get("id") == run["head_repository"]["id"]


def is_current_run(run):
    current = github_api(f"actions/runs/{run['id']}")
    if current["run_attempt"] != run["run_attempt"] or current["status"] != run["status"]:
        return False
    runs = github_items(f"actions/workflows/ci.yml/runs?event=pull_request&head_sha={run['head_sha']}&per_page=100",
                        "workflow_runs")
    return not any(item["id"] > run["id"] and item["head_branch"] == run["head_branch"]
                   and item["display_title"] == run["display_title"]
                   and matches_head_repository(run, item.get("head_repository"))
                   for item in runs)


def resolve_pr(run):
    # Fork runs can have no pull_requests. CI's run-name identifies the PR even
    # when one head branch is proposed against several base branches.
    match = PR_RUN_NAME.fullmatch(run.get("display_title", ""))
    if not match:
        return None
    pr = github_api(f"pulls/{match[1]}")
    if (pr["state"] == "open" and pr["base"]["ref"] == "dev"
            and pr["base"]["repo"]["full_name"] == REPOSITORY
            and pr["head"]["sha"] == run["head_sha"] and pr["head"]["ref"] == run["head_branch"]
            and matches_head_repository(run, pr["head"].get("repo"))):
        return pr
    return None


def output(**values):
    with open(os.environ["GITHUB_OUTPUT"], "a") as target:
        for key, value in values.items():
            if "\n" in str(value) or "\r" in str(value):
                raise ValueError(f"Invalid output {key}")
            target.write(f"{key}={value}\n")


def report(head_sha, state, description):
    url = f"https://github.com/{REPOSITORY}/actions/runs/{os.environ['GITHUB_RUN_ID']}"
    github_api(f"statuses/{head_sha}", {
        "state": state, "context": STATUS_CONTEXT, "description": description, "target_url": url,
    })


def write_candidate_outputs(run, pr):
    artifacts = github_items(f"actions/runs/{run['id']}/artifacts?per_page=100", "artifacts")
    candidates = [artifact for artifact in artifacts if artifact["name"].startswith("spxbench-pr-")]
    if len(candidates) != 1 or candidates[0]["expired"]:
        raise ValueError("Expected exactly one unexpired spxbench PR candidate")
    artifact = candidates[0]
    match = CANDIDATE_NAME.fullmatch(artifact["name"])
    if not match or int(match[1]) != pr["number"]:
        raise ValueError("Candidate artifact does not identify this PR")
    merge_sha, version = match[2], match[3]
    merge = github_api(f"git/commits/{merge_sha}")
    if len(merge["parents"]) != 2 or merge["parents"][1]["sha"] != run["head_sha"]:
        raise ValueError("Candidate commit is not a merge of this PR head")
    output(artifact_id=artifact["id"], target_sha=merge_sha, version=version)


def prepare():
    run = workflow_run("in_progress", "completed")
    if not run.get("head_repository") or not is_current_run(run):
        print("Ignoring superseded CI run")
        return
    pr = resolve_pr(run)
    if pr is None:
        print("No current open dev PR matches this CI run")
        return
    head_sha = run["head_sha"]
    if run["status"] == "in_progress":
        report(head_sha, "pending", "Waiting for CI to build the Web candidate")
        return

    output(head_sha=head_sha)
    report(head_sha, "pending", "Validating the CI Web candidate")
    if run["conclusion"] != "success":
        raise ValueError(f"Required CI run ended with {run['conclusion']}")
    write_candidate_outputs(run, pr)


def finish():
    run = workflow_run("completed")
    head_sha = os.environ["PR_HEAD_SHA"]
    if head_sha != run["head_sha"]:
        raise ValueError("Status target does not match the CI head")
    if not is_current_run(run) or resolve_pr(run) is None:
        print("Ignoring result for a superseded or closed PR")
        return
    if os.environ["PREPARE_RESULT"] != "success":
        state, description = "failure", "CI or candidate validation failed"
    elif os.environ["SUITE_RESULT"] == "success":
        state, description = "success", "spxbench regression suite passed"
    else:
        state, description = "failure", "spxbench regression suite failed or was interrupted"
    report(head_sha, state, description)
    print(f"spxbench: {state}: {description}")


if __name__ == "__main__":
    try:
        {"prepare": prepare, "finish": finish}[sys.argv[1]]()
    except (KeyError, ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"[error] {error}", file=sys.stderr)
        raise SystemExit(1)
