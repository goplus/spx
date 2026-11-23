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

import http.client
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_SPXBENCH_URL = "https://spxbench.qiniu.io"
DEFAULT_SPXBENCH_TIMEOUT_MINUTES = 30

BUSY_RETRY_SECONDS = 120
EVENT_HEARTBEAT_LOG_SECONDS = 60
EVENT_RECONNECT_INITIAL_SECONDS = 2
EVENT_RECONNECT_MAX_SECONDS = 10
EVENT_RECOVERY_TIMEOUT_SECONDS = 120
EVENT_STATE_REQUEST_TIMEOUT_SECONDS = 15
INSTALL_POLL_SECONDS = 5
INSTALL_REQUEST_TIMEOUT_SECONDS = 30
REMOVE_TIMEOUT_SECONDS = 120
REMOVE_RETRY_SECONDS = 10


class SpxbenchSuiteError(Exception):
    """A suite gate failure reported in the release workflow."""


class SpxbenchConnectionError(SpxbenchSuiteError):
    """A transient failure connecting to spxbench."""


def log(message):
    # Release-script convention: [info] on stdout, [error] on stderr.
    print(f"[info] {message}", flush=True)


def warn_cleanup(message):
    print(f"[warning] {message}", flush=True)
    escaped = str(message).replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")
    print(f"::warning title=spxbench cleanup::{escaped}", flush=True)


def run_command(*args, check=True):
    result = subprocess.run(args, text=True, capture_output=True)
    if check and result.returncode != 0:
        detail = (result.stderr or result.stdout or "").strip()
        raise SpxbenchSuiteError(f"command failed ({result.returncode}): {' '.join(args)}"
                              + (f": {detail}" if detail else ""))
    return result


def response_detail(body):
    return body if isinstance(body, str) else json.dumps(body)


def bounded_timeout(deadline, limit, timeout_message):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise SpxbenchSuiteError(timeout_message)
    return min(limit, remaining)


def http_error_body(error):
    # A lost response body must preserve the HTTP status for the caller's retry decision.
    with error:
        try:
            return error.read().decode(errors="replace")
        except (OSError, http.client.IncompleteRead):
            return "<unreadable response body>"


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
        body = http_error_body(error)
        try:
            return error.code, json.loads(body)
        except ValueError:
            return error.code, body
    except urllib.error.URLError as error:
        raise SpxbenchConnectionError(f"spxbench request failed: {error}") from error


def sse_events(response, deadline=None, on_comment=None):
    event, data = None, []
    for raw_line in response:
        if deadline is not None and time.monotonic() >= deadline:
            raise SpxbenchSuiteError("timed out waiting for spxbench job completion event")
        line = raw_line.decode("utf-8").rstrip("\r\n")
        if not line:
            if event is not None:
                yield event, "\n".join(data)
            event, data = None, []
        elif line.startswith("event:"):
            event = line[6:].strip()
        elif line.startswith("data:"):
            data.append(line[5:].lstrip())
        elif line.startswith(":") and on_comment is not None:
            on_comment(line[1:].strip())


def require_env(name):
    value = os.environ.get(name, "").strip()
    if not value:
        raise SpxbenchSuiteError(f"required environment variable {name} is not set")
    return value


def set_prerelease_cleanup_pending(pending):
    path = os.environ.get("GITHUB_OUTPUT")
    if path:
        with open(path, "a") as output:
            output.write(f"prerelease_cleanup_pending={str(pending).lower()}\n")


def github_context(name, *fallback_command):
    return os.environ.get(name, "").strip() or run_command(*fallback_command).stdout.strip()


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
    return "already in progress" in response_detail(body)


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
    """Run a candidate through an spxbench suite and manage its cleanup."""

    def __init__(self):
        self.spxbench_url = (os.environ.get("SPXBENCH_URL") or DEFAULT_SPXBENCH_URL).strip().rstrip("/")
        self.token = require_env("SPXBENCH_API_TOKEN")
        self.web_archive_path = require_env("WEB_ARCHIVE_PATH")
        self.candidate_version = require_env("CANDIDATE_VERSION")
        self.suite_name = os.environ.get("SPXBENCH_SUITE_NAME", "").strip() or "spx"
        try:
            self.timeout_minutes = int(os.environ.get("SPXBENCH_TIMEOUT_MINUTES", "").strip()
                                       or DEFAULT_SPXBENCH_TIMEOUT_MINUTES)
        except ValueError as error:
            raise SpxbenchSuiteError("SPXBENCH_TIMEOUT_MINUTES must be a positive integer") from error
        if self.timeout_minutes <= 0:
            raise SpxbenchSuiteError("SPXBENCH_TIMEOUT_MINUTES must be a positive integer")
        self.candidate_tag = f"v{self.candidate_version}"
        self.deadline = time.monotonic() + self.timeout_minutes * 60
        self.install_started = False
        self.candidate_removed = False

    def api_url(self, path):
        return f"{self.spxbench_url}{path}"

    def request(self, path, method="GET", payload=None, timeout=180):
        timeout = bounded_timeout(self.deadline, timeout, f"spxbench suite exceeded {self.timeout_minutes} minutes")
        return request_spxbench(self.api_url(path), self.token, method=method, payload=payload, timeout=timeout)

    def recoverable_request(self, path, description, **kwargs):
        """Return (None, detail) when the caller can recover from a lost response or 5xx."""
        try:
            status, body = self.request(path, **kwargs)
        except (SpxbenchConnectionError, OSError, http.client.IncompleteRead) as error:
            detail = str(error)
        else:
            if not 500 <= status < 600:
                return status, body
            detail = f"HTTP {status}: {body}"
        log(f"   {description} temporarily unavailable: {detail}")
        return None, detail

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
        repository = github_context("GITHUB_REPOSITORY", "gh", "repo", "view",
                                    "--json", "nameWithOwner", "-q", ".nameWithOwner")
        asset_url = f"https://github.com/{repository}/releases/download/{self.candidate_tag}/spx_web.zip"
        log(f"   candidate package: {asset_url}")
        log(f"installing version {self.candidate_version}")
        set_prerelease_cleanup_pending(False)
        self.install_started = True
        payload = {"kind": "spx", "version": self.candidate_version, "sourceUrl": asset_url}
        operation_id = f"spx:{self.candidate_version}"
        status, body = self.recoverable_request("/api/runtime-versions/install", "install request", method="POST",
                                                payload=payload, timeout=INSTALL_REQUEST_TIMEOUT_SECONDS)
        if status is None:
            # The POST may have been accepted. Recover by its deterministic operation ID.
            log(f"   recovering install operation {operation_id}")
            body = self.wait_for_candidate_install(operation_id, fallback_error=body)
        elif status == 202:
            accepted_id = body.get("operationId") if isinstance(body, dict) else None
            if not accepted_id:
                raise SpxbenchSuiteError(f"install response is missing operationId: {body}")
            if accepted_id != operation_id:
                raise SpxbenchSuiteError(
                    f"install response returned operationId {accepted_id!r}, want {operation_id!r}")
            log(f"   install accepted; operation={operation_id}")
            body = self.wait_for_candidate_install(operation_id)
        elif status != 200:
            raise SpxbenchSuiteError(f"install request failed: HTTP {status}: {body}")

        # An idempotent 200 and a completed async install both return the registered versions.
        artifact = self.candidate_artifact(body)
        if artifact is None:
            raise SpxbenchSuiteError(f"install response did not include candidate {self.candidate_version}")
        log(f"   installed, digest={artifact.get('digest')}")

    def candidate_artifact(self, body):
        if not isinstance(body, dict):
            return None
        return next((item for item in body.get("versions") or []
                     if isinstance(item, dict) and item.get("kind") == "spx"
                     and item.get("version") == self.candidate_version), None)

    def pause_install_poll(self):
        time.sleep(bounded_timeout(self.deadline, INSTALL_POLL_SECONDS,
                                   f"timed out installing candidate {self.candidate_version}"))

    def read_install_status(self, path, description):
        while True:
            status, body = self.recoverable_request(path, description, timeout=INSTALL_REQUEST_TIMEOUT_SECONDS)
            if status is not None:
                return status, body
            self.pause_install_poll()

    def wait_for_candidate_install(self, operation_id, fallback_error=None):
        """Wait for the async operation, then read the registered versions."""
        path = f"/api/runtime-versions/operations/{urllib.parse.quote(operation_id, safe='')}"
        while True:
            status, body = self.read_install_status(path, "install status")
            if status == 404:
                detail = f"install operation {operation_id} was not found"
                if fallback_error:
                    detail = f"{fallback_error}; {detail}"
                raise SpxbenchSuiteError(detail)
            if status != 200:
                raise SpxbenchSuiteError(f"install status failed: HTTP {status}: {body}")
            operation_status = str(body.get("status") or "").lower() if isinstance(body, dict) else ""
            if operation_status == "success":
                break
            if operation_status == "failed":
                raise SpxbenchSuiteError(f"install failed: {body.get('error') or 'unknown error'}")
            if operation_status != "running":
                raise SpxbenchSuiteError(f"install operation has unexpected status {operation_status!r}: {body}")
            log(f"   install still running; stage={body.get('stage') or '(unknown)'}")
            self.pause_install_poll()

        status, versions = self.read_install_status("/api/runtime-versions", "installed candidate lookup")
        if status != 200:
            raise SpxbenchSuiteError(f"read installed candidate failed: HTTP {status}: {versions}")
        return versions

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
            if status != 409:
                raise SpxbenchSuiteError(f"run failed: HTTP {status}: {body}")
            if not is_run_slot_busy(body):
                raise SpxbenchSuiteError(f"run rejected: {response_detail(body).strip()}")
            delay = bounded_timeout(self.deadline, BUSY_RETRY_SECONDS, "timed out waiting for a free run slot")
            log(f"   run slot busy (409), retrying in {delay:.0f}s")
            time.sleep(delay)

    def wait_for_suite_job(self, job_id):
        log(f"waiting for job #{job_id} completion event")
        url = self.api_url(f"/__e2e/api/job-events?job={job_id}")
        request = urllib.request.Request(url, headers={"Authorization": f"Bearer {self.token}"})
        wait_started = time.monotonic()
        last_wait_log = wait_started
        recovery_deadline = None
        reconnect_delay = EVENT_RECONNECT_INITIAL_SECONDS

        def on_comment(comment):
            nonlocal last_wait_log, recovery_deadline, reconnect_delay
            if comment != "heartbeat":
                return
            now = time.monotonic()
            recovery_deadline = None
            reconnect_delay = EVENT_RECONNECT_INITIAL_SECONDS
            if now - last_wait_log >= EVENT_HEARTBEAT_LOG_SECONDS:
                log(f"   job #{job_id} still running; waiting for completion event ({int(now - wait_started)}s)")
                last_wait_log = now

        def stream_timeout(limit):
            limit = bounded_timeout(self.deadline, limit,
                                    f"job #{job_id} did not finish within {self.timeout_minutes} minutes")
            if recovery_deadline is None:
                return limit
            return bounded_timeout(recovery_deadline, limit,
                                   f"job #{job_id} event stream unavailable for {EVENT_RECOVERY_TIMEOUT_SECONDS} seconds")

        while True:
            finished_event = False
            try:
                with urllib.request.urlopen(request, timeout=stream_timeout(45)) as response:
                    if response.headers.get_content_type() != "text/event-stream":
                        raise SpxbenchSuiteError("spxbench job event endpoint did not return an event stream")
                    finished_event = any(event == "finished" and json.loads(data).get("jobId") == job_id
                                         for event, data in sse_events(response, self.deadline, on_comment))
            except urllib.error.HTTPError as error:
                detail = http_error_body(error)
                if not 500 <= error.code < 600:
                    raise SpxbenchSuiteError(f"job event stream failed: HTTP {error.code}: {detail}") from error
                log(f"   job event stream temporarily unavailable: HTTP {error.code}: {detail}")
            except (OSError, http.client.IncompleteRead) as error:
                log(f"   job event stream disconnected: {error}")
            # Read durable state after an event or disconnect to cover lost notifications.
            if not finished_event and recovery_deadline is None:
                recovery_deadline = time.monotonic() + EVENT_RECOVERY_TIMEOUT_SECONDS
            state = self.read_suite_job_state(job_id, timeout=stream_timeout(EVENT_STATE_REQUEST_TIMEOUT_SECONDS))
            if state is not None:
                if not state.get("running"):
                    return state
                if finished_event:
                    raise SpxbenchSuiteError(f"job #{job_id} completion event did not match a finished state")
            elif recovery_deadline is None:
                recovery_deadline = time.monotonic() + EVENT_RECOVERY_TIMEOUT_SECONDS
            delay = stream_timeout(reconnect_delay)
            log(f"   reconnecting job event stream in {delay:.0f}s")
            time.sleep(delay)
            reconnect_delay = min(reconnect_delay * 2, EVENT_RECONNECT_MAX_SECONDS)

    def read_suite_job_state(self, job_id, timeout=EVENT_STATE_REQUEST_TIMEOUT_SECONDS):
        status, job = self.recoverable_request(f"/api/jobs?id={job_id}", "job state", timeout=timeout)
        if status is None:
            return None
        if status != 200:
            raise SpxbenchSuiteError(f"state read failed: HTTP {status}: {job}")
        return self.state_from_job_response(job_id, job)

    @staticmethod
    def state_from_job_response(job_id, job):
        """Adapt the durable /api/jobs response to the suite gate state shape."""
        if not isinstance(job, dict):
            raise SpxbenchSuiteError(f"state read returned an invalid job: {job}")
        if job.get("id") != job_id:
            raise SpxbenchSuiteError(f"state read returned a different job: {job.get('id')}")
        snapshot = job.get("snapshot")
        if not isinstance(snapshot, dict):
            raise SpxbenchSuiteError(f"state read returned no snapshot for job {job_id}")

        state = dict(snapshot)
        state["jobId"] = job_id
        state["running"] = job.get("status") == "running"
        for field in ("total", "completed", "passed", "failed", "skipped", "summary"):
            if field in job:
                state[field] = job[field]
        if "error" in job:
            state["lastError"] = job["error"]
        return state

    def check_suite_result(self, job_id, state):
        # The job's own terminal verdict decides; the case list only adds detail.
        total, passed, failed = state.get("total"), state.get("passed"), state.get("failed")
        log(f"result: summary={state.get('summary') or '(none)'} total={total} "
            f"passed={passed} failed={failed}")
        log(f"   details: {self.api_url(f'/__e2e?job={job_id}')}")
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

    def remove_candidate(self):
        log(f"removing passed candidate {self.candidate_version} from spxbench")
        deadline = time.monotonic() + REMOVE_TIMEOUT_SECONDS
        payload = {"action": "remove", "kind": "spx", "version": self.candidate_version}
        while True:
            timeout = bounded_timeout(deadline, 30,
                                      f"timed out removing candidate {self.candidate_version} from spxbench")
            status, body = request_spxbench(self.api_url("/api/runtime-versions"), self.token,
                                            method="POST", payload=payload, timeout=timeout)
            if status == 200:
                versions = body.get("versions") if isinstance(body, dict) else None
                if not isinstance(versions, list) or not all(isinstance(item, dict) for item in versions):
                    raise SpxbenchSuiteError(f"invalid candidate removal response: {body}")
                if self.candidate_artifact(body) is not None:
                    raise SpxbenchSuiteError(f"spxbench still lists candidate {self.candidate_version} after removal")
                break
            if status == 404:
                break
            if status != 409:
                raise SpxbenchSuiteError(f"remove candidate failed: HTTP {status}: {body}")
            delay = bounded_timeout(deadline, REMOVE_RETRY_SECONDS,
                                    f"timed out removing candidate {self.candidate_version}: {body}")
            log(f"   candidate removal blocked (409), retrying in {delay:.0f}s")
            time.sleep(delay)
        self.candidate_removed = True
        set_prerelease_cleanup_pending(True)
        log(f"   removed candidate {self.candidate_version} from spxbench")

    def write_summary(self, suite_result, runtime_cleanup, release_cleanup, job_id, cleanup_notes):
        path = os.environ.get("GITHUB_STEP_SUMMARY")
        if not path:
            return
        lines = ["### spxbench regression suite", "",
                 f"- Suite: `{self.suite_name}`",
                 f"- Candidate: `{self.candidate_version}`",
                 f"- Regression: **{suite_result}**",
                 f"- spxbench candidate: **{runtime_cleanup}**",
                 f"- GitHub prerelease: **{release_cleanup}**"]
        if job_id is not None:
            lines.append(f"- [Test run details]({self.api_url(f'/__e2e?job={job_id}')})")
        lines.extend(f"- Cleanup warning: {' '.join(str(note).splitlines())}"
                     for note in cleanup_notes)
        try:
            with open(path, "a", encoding="utf-8") as summary:
                summary.write("\n".join(lines) + "\n\n")
        except OSError as error:
            warn_cleanup(f"writing GitHub Step Summary failed: {error}")

    def run(self):
        suite_result = "not completed"
        runtime_cleanup = "not installed"
        cleanup_notes = []
        job_id = None
        commit = github_context("GITHUB_SHA", "git", "rev-parse", "HEAD")
        # gh may fail or be cancelled after creating the prerelease.
        set_prerelease_cleanup_pending(True)
        try:
            create_candidate_release(self.candidate_tag, self.web_archive_path, commit)
            suite_id = self.resolve_suite()
            self.install_candidate()
            job_id = self.start_suite_run(suite_id)
            state = self.wait_for_suite_job(job_id)
            suite_result = "failed"
            self.check_suite_result(job_id, state)
            suite_result = "passed"
            try:
                self.remove_candidate()
            except Exception as error:
                cleanup_notes.append(f"spxbench candidate removal: {error}")
                warn_cleanup(f"spxbench candidate removal failed: {error}")
            runtime_cleanup = "removed" if self.candidate_removed else "failed"
        finally:
            if self.install_started and not self.candidate_removed:
                if suite_result != "passed":
                    runtime_cleanup = "retained"
                release_cleanup = "retained"
                log(f"retaining candidate {self.candidate_version} and its prerelease for investigation")
            else:
                try:
                    deletion_error = delete_candidate_release(self.candidate_tag)
                except Exception as error:
                    deletion_error = str(error)
                if deletion_error is not None:
                    release_cleanup = "failed"
                    cleanup_notes.append(f"GitHub prerelease deletion: {deletion_error}")
                    warn_cleanup(f"GitHub prerelease cleanup failed: {deletion_error}")
                else:
                    release_cleanup = "deleted"
                    try:
                        set_prerelease_cleanup_pending(False)
                    except OSError as error:
                        warn_cleanup(f"recording prerelease deletion failed: {error}")
            self.write_summary(suite_result, runtime_cleanup, release_cleanup, job_id, cleanup_notes)


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
