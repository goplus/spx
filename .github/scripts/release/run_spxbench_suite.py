#!/usr/bin/env python3
"""Check a release candidate against recorded results with an spxbench suite.

Required environment:
  SPXBENCH_API_TOKEN  spxbench admin bearer token
  WEB_ARCHIVE_PATH    path to the built spx_web.zip
  CANDIDATE_VERSION   candidate version, e.g. 4.0.2-rc.43192600496.1
Optional:
  SPXBENCH_URL        spxbench base URL (default: https://spxbench.qiniu.io)
  SPXBENCH_SUITE_NAME gate suite name (default: spx; cross-repo contract)
  SPXBENCH_TIMEOUT_MINUTES overall budget (default: 30)
  GITHUB_REPOSITORY / GITHUB_SHA  provided by Actions; fall back to gh/git
"""

import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_SPXBENCH_URL = "https://spxbench.qiniu.io"

POLL_INTERVAL_SECONDS = 30
BUSY_RETRY_SECONDS = 120


class SpxbenchSuiteError(Exception):
    """A suite gate failure reported in the release workflow."""


def log(message):
    # Release-script convention: [info] on stdout, [error] on stderr.
    print(f"[info] {message}", flush=True)


def run_command(*args, check=True):
    result = subprocess.run(args, text=True, capture_output=True)
    if check and result.returncode != 0:
        detail = (result.stderr or result.stdout or "").strip()
        raise SpxbenchSuiteError(f"command failed ({result.returncode}): {' '.join(args)}"
                              + (f": {detail}" if detail else ""))
    return result


def request_spxbench(url, token, method="GET", payload=None, timeout=180):
    """Return (http_status, parsed_body) for one spxbench API call."""
    request = urllib.request.Request(url, method=method)
    request.add_header("Authorization", f"Bearer {token}")
    data = None
    if payload is not None:
        request.add_header("Content-Type", "application/json")
        data = json.dumps(payload).encode()
    try:
        with urllib.request.urlopen(request, data=data, timeout=timeout) as response:
            body = response.read().decode()
            return response.status, (json.loads(body) if body else {})
    except urllib.error.HTTPError as error:
        body = error.read().decode()
        try:
            return error.code, json.loads(body)
        except ValueError:
            return error.code, body
    except urllib.error.URLError as error:
        raise SpxbenchSuiteError(f"spxbench request failed: {error}") from error


def require_env(name):
    value = os.environ.get(name, "").strip()
    if not value:
        raise SpxbenchSuiteError(f"required environment variable {name} is not set")
    return value


def github_repository():
    env_value = os.environ.get("GITHUB_REPOSITORY", "").strip()
    if env_value:
        return env_value
    return run_command("gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner").stdout.strip()


def source_commit_sha():
    env_value = os.environ.get("GITHUB_SHA", "").strip()
    if env_value:
        return env_value
    return run_command("git", "rev-parse", "HEAD").stdout.strip()


def find_suite_id(suites, name):
    for suite in suites or []:
        if suite.get("name") == name:
            return suite.get("id")
    return None


def failing_cases(state):
    return [case for case in state.get("cases") or [] if case.get("status") != "pass"]


def job_failure_reasons(state):
    """Return all reasons a finished job cannot pass the release gate.

    The per-case list alone is not the verdict: spxbench also fails a job on
    process-level errors (summary/lastError) even when every case passed.
    """
    reasons = []
    summary = (state.get("summary") or "").strip()
    if summary != "passed":
        reasons.append(f"job summary is {summary!r}, want 'passed'")
    last_error = (state.get("lastError") or "").strip()
    if last_error:
        reasons.append(f"job error: {last_error}")
    total, completed = state.get("total") or 0, state.get("completed") or 0
    if total <= 0:
        reasons.append("job selected no test cases")
    if completed != total:
        reasons.append(f"job incomplete: completed={completed} of total={total}")
    # spxbench can report "passed" when a case was skipped.
    passed = state.get("passed")
    if passed != total:
        reasons.append(f"job passed {passed} of {total} cases")
    failures = failing_cases(state)
    if failures:
        reasons.append(f"{len(failures)} case(s) did not pass")
    return reasons


def is_run_slot_busy(body):
    """True only for the recoverable occupied-slot 409; other 409s are permanent."""
    detail = body if isinstance(body, str) else json.dumps(body)
    return "already in progress" in detail


def create_candidate_release(candidate_tag, web_archive_path, target_sha):
    log(f"creating candidate prerelease {candidate_tag}")
    run_command("gh", "release", "create", candidate_tag, f"{web_archive_path}#spx_web.zip",
        "--title", f"SPX {candidate_tag[1:]} (spxbench suite candidate)",
        "--notes", "Temporary candidate for the spxbench suite; cleanup runs after the gate.",
        "--prerelease", "--target", target_sha)


def delete_candidate_release(candidate_tag):
    log(f"deleting candidate prerelease {candidate_tag}")
    try:
        result = run_command("gh", "release", "delete", candidate_tag, "--cleanup-tag", "--yes", check=False)
    except OSError as error:
        failure = str(error)
    else:
        if result.returncode == 0:
            return None
        failure = (result.stderr or result.stdout or "").strip() or f"gh exited {result.returncode}"
    log(f"candidate prerelease deletion failed: {failure}")
    return failure


class SpxbenchSuiteGate:
    """Run a candidate through an spxbench suite and clean up its release."""

    def __init__(self):
        self.spxbench_url = (os.environ.get("SPXBENCH_URL") or DEFAULT_SPXBENCH_URL).strip().rstrip("/")
        self.token = require_env("SPXBENCH_API_TOKEN")
        self.web_archive_path = require_env("WEB_ARCHIVE_PATH")
        self.candidate_version = require_env("CANDIDATE_VERSION")
        self.suite_name = os.environ.get("SPXBENCH_SUITE_NAME", "").strip() or "spx"
        try:
            self.timeout_minutes = int(os.environ.get("SPXBENCH_TIMEOUT_MINUTES", "").strip() or 30)
        except ValueError as error:
            raise SpxbenchSuiteError("SPXBENCH_TIMEOUT_MINUTES must be a positive integer") from error
        if self.timeout_minutes <= 0:
            raise SpxbenchSuiteError("SPXBENCH_TIMEOUT_MINUTES must be a positive integer")
        self.candidate_tag = f"v{self.candidate_version}"
        self.deadline = time.monotonic() + self.timeout_minutes * 60

    def request(self, path, method="GET", payload=None):
        remaining = self.deadline - time.monotonic()
        if remaining <= 0:
            raise SpxbenchSuiteError(f"spxbench suite exceeded {self.timeout_minutes} minutes")
        return request_spxbench(f"{self.spxbench_url}{path}", self.token, method=method,
                                payload=payload, timeout=min(180, remaining))

    def resolve_suite(self):
        # ids are per-database, names are the cross-repo contract.
        log(f"looking up suite {self.suite_name!r}")
        status, body = self.request("/api/suites")
        if status != 200:
            raise SpxbenchSuiteError(f"list suites failed: HTTP {status}: {body}")
        suite_id = find_suite_id(body.get("suites"), self.suite_name)
        if suite_id is None:
            available = ", ".join(sorted((s.get("name") or "") for s in body.get("suites") or []))
            raise SpxbenchSuiteError(f"no suite named {self.suite_name!r}; available: {available or '(none)'}")
        log(f"   suite id={suite_id}")
        return suite_id

    def install_candidate(self):
        asset_url = (f"https://github.com/{github_repository()}"
                     f"/releases/download/{self.candidate_tag}/spx_web.zip")
        log(f"   candidate package: {asset_url}")
        log(f"installing version {self.candidate_version}")
        status, body = self.request("/api/runtime-versions", method="POST",
                                payload={"action": "install", "kind": "spx",
                                         "version": self.candidate_version, "sourceUrl": asset_url})
        if status != 200:
            raise SpxbenchSuiteError(f"install failed: HTTP {status}: {body}")
        artifact = next((item for item in body.get("versions", [])
                         if item.get("kind") == "spx" and item.get("version") == self.candidate_version), None)
        if artifact is None:
            raise SpxbenchSuiteError(f"install response did not include candidate {self.candidate_version}")
        log(f"   installed, digest={artifact.get('digest')}")

    def start_suite_run(self, suite_id):
        log(f"starting suite run (suite={suite_id}, spxVersion={self.candidate_version})")
        query = urllib.parse.urlencode({"suite": suite_id, "spxVersion": self.candidate_version})
        while True:
            if time.monotonic() >= self.deadline:
                raise SpxbenchSuiteError("timed out waiting for a free run slot")
            status, body = self.request(f"/__e2e/api/run?{query}", method="POST")
            if status == 200:
                job_id = body.get("jobId")
                if not job_id:
                    raise SpxbenchSuiteError(f"run response is missing jobId: {body}")
                log(f"   job #{job_id} accepted")
                return job_id
            elif status == 409:
                if not is_run_slot_busy(body):
                    detail = body if isinstance(body, str) else json.dumps(body)
                    raise SpxbenchSuiteError(f"run rejected: {detail.strip()}")
                remaining = self.deadline - time.monotonic()
                if remaining <= 0:
                    raise SpxbenchSuiteError("timed out waiting for a free run slot")
                delay = min(BUSY_RETRY_SECONDS, remaining)
                log(f"   run slot busy (409), retrying in {delay:.0f}s")
                time.sleep(delay)
            else:
                raise SpxbenchSuiteError(f"run failed: HTTP {status}: {body}")

    def wait_for_suite_job(self, job_id):
        log(f"polling job state (every {POLL_INTERVAL_SECONDS}s)")
        while True:
            status, state = self.request(f"/__e2e/api/state?job={job_id}")
            if status != 200:
                raise SpxbenchSuiteError(f"state read failed: HTTP {status}: {state}")
            remaining = self.deadline - time.monotonic()
            if remaining <= 0:
                raise SpxbenchSuiteError(f"job #{job_id} did not finish within {self.timeout_minutes} minutes")
            if not state.get("running") and state.get("jobId") == job_id:
                return state
            log(f"   progress {state.get('completed')}/{state.get('total')} "
                f"passed={state.get('passed')} failed={state.get('failed')}")
            time.sleep(min(POLL_INTERVAL_SECONDS, remaining))

    def check_suite_result(self, job_id, state):
        # The job's own terminal verdict decides; the case list only adds detail.
        total, passed, failed = state.get("total"), state.get("passed"), state.get("failed")
        log(f"result: summary={state.get('summary') or '(none)'} total={total} "
            f"passed={passed} failed={failed}")
        log(f"   details: {self.spxbench_url}/__e2e?job={job_id}")
        failures = failing_cases(state)
        if failures:
            log("   failing cases:")
            for case in failures:
                log(f"     - {case.get('name')} [{case.get('status')}]")
        reasons = job_failure_reasons(state)
        if reasons:
            raise SpxbenchSuiteError(f"candidate {self.candidate_version} failed suite {self.suite_name!r}: "
                                  + "; ".join(reasons))
        log(f"   candidate {self.candidate_version} passed suite {self.suite_name!r} ({total} cases)")

    def run(self):
        completed = False
        try:
            # Creation is inside the try: gh release create is multi-step and can
            # fail after the remote release already exists; finally must clean it.
            create_candidate_release(self.candidate_tag, self.web_archive_path, source_commit_sha())
            suite_id = self.resolve_suite()
            self.install_candidate()
            job_id = self.start_suite_run(suite_id)
            state = self.wait_for_suite_job(job_id)
            self.check_suite_result(job_id, state)
            completed = True
        finally:
            deletion_error = delete_candidate_release(self.candidate_tag)
            if completed and deletion_error:
                raise SpxbenchSuiteError(f"spxbench suite passed, but release cleanup failed: {deletion_error}")


def main():
    SpxbenchSuiteGate().run()


if __name__ == "__main__":
    try:
        main()
    except (SpxbenchSuiteError, OSError, ValueError) as error:
        # Surface the failure as a GitHub annotation, like plan.py does.
        print(f"::error title=run-spxbench-suite::{error}", flush=True)
        print(f"[error] {error}", file=sys.stderr, flush=True)
        raise SystemExit(1)
