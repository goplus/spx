"""Unit tests for the spxbench release suite gate."""

import hashlib
import http.client
import io
import json
import os
import tempfile
import unittest
import urllib.error
import urllib.parse
from contextlib import ExitStack
from pathlib import Path
from unittest import mock

import run_spxbench_suite
from run_spxbench_suite import (SpxbenchConnectionError, SpxbenchSuiteError, SpxbenchSuiteGate,
                                find_suite_id, require_env, sse_events)


VERSION = "0.0.1-rc.1.1"
COMMIT = "0123456789abcdef0123456789abcdef01234567"
CANDIDATE_ENV = {
    "SPXBENCH_URL": "http://127.0.0.1", "SPXBENCH_API_TOKEN": "t",
    "WEB_ARCHIVE_PATH": "dist/spx_web.zip", "CANDIDATE_VERSION": VERSION,
    "SPXBENCH_ARTIFACT_ID": "123", "SPXBENCH_ARTIFACT_RUN_ID": "42",
    "GITHUB_REPOSITORY": "goplus/spx", "GITHUB_SHA": COMMIT,
}
ARTIFACT_URL = "https://api.github.com/repos/goplus/spx/actions/artifacts/123/zip"
SUITE_LIST = (200, {"suites": [{"id": 7, "name": "spx"}]})
OPERATION_ID = f"spx:{VERSION}"
INSTALL_ACCEPTED = (202, {"operationId": OPERATION_ID, "status": "running"})
INSTALL_SUCCEEDED = (200, {"operationId": OPERATION_ID, "status": "success"})
ARCHIVE = b"candidate Web archive"
DIGEST = "sha256:" + hashlib.sha256(ARCHIVE).hexdigest()
CANDIDATE = {"kind": "spx", "version": VERSION, "sourceUrl": ARTIFACT_URL, "sourceCommit": COMMIT, "digest": DIGEST}
CANDIDATE_REGISTERED = (200, {"versions": [CANDIDATE]})
RUN_PATH = f"/__e2e/api/run?suite=7&spxVersion={VERSION}&trigger=spx-dry-run&repository=goplus%2Fspx"
INSTALL_PATH = "/api/runtime-versions/install?" + urllib.parse.urlencode({
    "kind": "spx", "version": VERSION, "sourceUrl": ARTIFACT_URL, "sourceCommit": COMMIT, "sourceDigest": DIGEST})
INSTALL_REQUEST = mock.call(INSTALL_PATH, method="POST", archive=mock.ANY)
OPERATION_PATH = f"/api/runtime-versions/operations/spx%3A{VERSION}"
OPERATION_REQUEST = mock.call(OPERATION_PATH, timeout=30)
VERSIONS_REQUEST = mock.call("/api/runtime-versions", timeout=30)


def patch_candidate_env(**updates):
    return mock.patch.dict(os.environ, {**CANDIDATE_ENV, **updates}, clear=True)


def job_response(state, *, status=None, job_id=11):
    """Build the durable /api/jobs response returned by spxbench."""
    snapshot = dict(state)
    snapshot["jobId"] = job_id
    if status is None:
        status = "running" if state.get("running") else "success"
    return {
        "id": job_id,
        "status": status,
        "total": state.get("total", 0),
        "completed": state.get("completed", 0),
        "passed": state.get("passed", 0),
        "failed": state.get("failed", 0),
        "skipped": state.get("skipped", 0),
        "summary": state.get("summary", ""),
        "error": state.get("lastError", ""),
        "snapshot": snapshot,
    }


class FindSuiteIdTest(unittest.TestCase):
    def test_matches_by_exact_name(self):
        suites = [{"id": 7, "name": "spx"}, {"id": 9, "name": "smoke"}]
        self.assertEqual(find_suite_id(suites, "spx"), 7)
        self.assertEqual(find_suite_id(suites, "smoke"), 9)

    def test_missing_name_returns_none(self):
        self.assertIsNone(find_suite_id([{"id": 1, "name": "spx"}], "ghost"))
        self.assertIsNone(find_suite_id(None, "spx"))
        self.assertIsNone(find_suite_id([], "spx"))


class RequireEnvTest(unittest.TestCase):
    def test_missing_variable_raises(self):
        with mock.patch.dict(os.environ, {}, clear=True):
            with self.assertRaises(SpxbenchSuiteError):
                require_env("SPXBENCH_SUITE_TEST_VAR")

    def test_blank_value_raises(self):
        with mock.patch.dict(os.environ, {"SPXBENCH_SUITE_TEST_VAR": "   "}):
            with self.assertRaises(SpxbenchSuiteError):
                require_env("SPXBENCH_SUITE_TEST_VAR")

    def test_value_is_trimmed(self):
        with mock.patch.dict(os.environ, {"SPXBENCH_SUITE_TEST_VAR": " x "}):
            self.assertEqual(require_env("SPXBENCH_SUITE_TEST_VAR"), "x")


@patch_candidate_env()
class JobVerdictTest(unittest.TestCase):
    def test_failed_or_cancelled_job_with_all_green_counts_never_passes(self):
        for status in ("failed", "cancelled"):
            job = job_response({"total": 1, "completed": 1, "passed": 1, "summary": "passed",
                                "cases": [{"name": "smoke", "status": "pass"}]}, status=status)
            with self.subTest(status=status), self.assertRaisesRegex(SpxbenchSuiteError, status):
                SpxbenchSuiteGate().check_suite_result(11, job)


@patch_candidate_env()
class SuiteRunConflictTest(unittest.TestCase):
    def test_retries_same_run_after_lost_response_and_busy_slot(self):
        responses = [SpxbenchConnectionError("lost response"), (503, "unavailable"),
                     (409, {"code": "run_slot_busy"}), (200, {"jobId": 11})]
        with mock.patch.object(SpxbenchSuiteGate, "request", side_effect=responses) as requested, \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().start_suite_run(7), 11)
            self.assertEqual(requested.call_args_list, [mock.call(RUN_PATH, method="PUT")] * 4)
            self.assertEqual(sleep.call_args_list, [mock.call(2), mock.call(2), mock.call(120)])

    def test_permanent_rejection_or_invalid_id_does_not_retry(self):
        for response in ((409, "a test run is already in progress"), (405, "old server"), (200, {"jobId": False})):
            with self.subTest(response=response), \
                 mock.patch.object(SpxbenchSuiteGate, "request", return_value=response) as requested:
                with self.assertRaises(SpxbenchSuiteError):
                    SpxbenchSuiteGate().start_suite_run(7)
                requested.assert_called_once_with(RUN_PATH, method="PUT")


class CandidateInstallTest(unittest.TestCase):
    def setUp(self):
        stack = ExitStack()
        self.addCleanup(stack.close)
        archive = Path(stack.enter_context(tempfile.TemporaryDirectory())) / "spx_web.zip"
        archive.write_bytes(ARCHIVE)
        stack.enter_context(patch_candidate_env(WEB_ARCHIVE_PATH=str(archive)))
        self.request = stack.enter_context(mock.patch.object(SpxbenchSuiteGate, "request"))
        self.sleep = stack.enter_context(mock.patch.object(run_spxbench_suite.time, "sleep"))

    def test_invalid_artifact_id_fails_before_install(self):
        for artifact_id in ("", "0", "01", "-1", "123/zip"):
            with self.subTest(artifact_id=artifact_id), patch_candidate_env(SPXBENCH_ARTIFACT_ID=artifact_id):
                with self.assertRaisesRegex(SpxbenchSuiteError, "SPXBENCH_ARTIFACT_ID"):
                    SpxbenchSuiteGate()
        self.request.assert_not_called()

    def test_already_completed_install_is_accepted(self):
        self.request.return_value = CANDIDATE_REGISTERED
        SpxbenchSuiteGate().install_candidate(COMMIT)
        self.assertEqual(self.request.call_args_list, [INSTALL_REQUEST])

    def test_registered_candidate_must_match_artifact_commit_and_digest(self):
        for field, value in (("sourceCommit", "f" * 40), ("sourceUrl", ARTIFACT_URL.replace("123", "124")),
                             ("digest", None), ("digest", "sha256:other")):
            self.request.return_value = (200, {"versions": [{**CANDIDATE, field: value}]})
            with self.subTest(field=field, value=value), self.assertRaisesRegex(SpxbenchSuiteError, field):
                SpxbenchSuiteGate().install_candidate(COMMIT)

    def test_invalid_install_response_is_rejected_without_polling(self):
        for response, message in (
            ((200, {"versions": []}), "did not include candidate"),
            ((202, {"status": "running"}), "returned operationId None"),
            ((202, {"operationId": "spx:another-version"}), "returned operationId"),
        ):
            with self.subTest(response=response):
                self.request.reset_mock()
                self.request.return_value = response
                with self.assertRaisesRegex(SpxbenchSuiteError, message):
                    SpxbenchSuiteGate().install_candidate(COMMIT)
                self.assertEqual(self.request.call_args_list, [INSTALL_REQUEST])

    def test_async_install_polls_until_candidate_is_registered(self):
        running = (200, {"operationId": OPERATION_ID, "status": "running", "stage": "下载 SPX archive"})
        self.request.side_effect = [INSTALL_ACCEPTED, running, INSTALL_SUCCEEDED, CANDIDATE_REGISTERED]
        SpxbenchSuiteGate().install_candidate(COMMIT)
        self.assertEqual(self.request.call_args_list,
                         [INSTALL_REQUEST, OPERATION_REQUEST, OPERATION_REQUEST, VERSIONS_REQUEST])
        self.sleep.assert_called_once_with(5)

    def test_unavailable_accept_response_recovers_existing_operation(self):
        for failure in (SpxbenchConnectionError("connection lost"), (504, "gateway timeout")):
            with self.subTest(failure=failure):
                self.request.reset_mock()
                self.request.side_effect = [failure, INSTALL_SUCCEEDED, CANDIDATE_REGISTERED]
                SpxbenchSuiteGate().install_candidate(COMMIT)
                self.assertEqual(self.request.call_args_list, [INSTALL_REQUEST, OPERATION_REQUEST, VERSIONS_REQUEST])
                self.sleep.assert_not_called()

    def test_unclaimed_upload_is_retried_only_once(self):
        self.request.side_effect = [(502, "lost"), (404, "not found")] * 2
        with self.assertRaisesRegex(SpxbenchSuiteError, "upload failed: HTTP 502.*was not found"):
            SpxbenchSuiteGate().install_candidate(COMMIT)
        self.assertEqual(self.request.call_args_list, [INSTALL_REQUEST, OPERATION_REQUEST] * 2)

    def test_final_candidate_lookup_retries_transient_failure(self):
        self.request.side_effect = [INSTALL_ACCEPTED, INSTALL_SUCCEEDED,
                                    (503, "temporarily unavailable"), CANDIDATE_REGISTERED]
        SpxbenchSuiteGate().install_candidate(COMMIT)
        self.assertEqual(self.request.call_args_list,
                         [INSTALL_REQUEST, OPERATION_REQUEST, VERSIONS_REQUEST, VERSIONS_REQUEST])
        self.sleep.assert_called_once_with(5)

    def test_async_install_failure_is_reported_without_retrying_post(self):
        failed = (200, {"operationId": OPERATION_ID, "status": "failed", "error": "download failed"})
        self.request.side_effect = [INSTALL_ACCEPTED, failed]
        with self.assertRaisesRegex(SpxbenchSuiteError, "download failed"):
            SpxbenchSuiteGate().install_candidate(COMMIT)
        self.assertEqual(self.request.call_args_list, [INSTALL_REQUEST, OPERATION_REQUEST])
        self.sleep.assert_not_called()


class CandidateUploadTest(unittest.TestCase):
    def test_disconnected_upload_resends_complete_archive_after_lookup(self):
        uploads = []

        def open_request(request, *, timeout):
            if request.get_method() == "GET":
                self.assertEqual(request.full_url, "http://127.0.0.1" + OPERATION_PATH)
                raise urllib.error.HTTPError(request.full_url, 404, "not found", {}, io.BytesIO())
            self.assertEqual(request.full_url, "http://127.0.0.1" + INSTALL_PATH)
            self.assertEqual(request.get_header("Authorization"), "Bearer t")
            self.assertEqual(request.get_header("Content-type"), "application/zip")
            self.assertEqual(request.get_header("Content-length"), str(len(ARCHIVE)))
            uploads.append(request.data.read())
            if len(uploads) == 1:
                raise urllib.error.URLError("connection lost")
            response = io.BytesIO(json.dumps(CANDIDATE_REGISTERED[1]).encode())
            response.status = 200
            return response

        with tempfile.NamedTemporaryFile() as archive, patch_candidate_env(WEB_ARCHIVE_PATH=archive.name), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=open_request):
            archive.write(ARCHIVE)
            archive.flush()
            SpxbenchSuiteGate().install_candidate(COMMIT)
        self.assertEqual(uploads, [ARCHIVE, ARCHIVE])


class CandidateRemovalTest(unittest.TestCase):
    def test_conflict_retries_then_removes_candidate(self):
        responses = [(409, {"error": "runtime version is in use"}), (200, {"versions": []})]
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite, "request_spxbench", side_effect=responses) as requested, \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            gate = SpxbenchSuiteGate()
            gate.remove_candidate()
        self.assertTrue(gate.candidate_removed)
        self.assertEqual(requested.call_count, 2)
        self.assertEqual(requested.call_args.kwargs["payload"],
                         {"action": "remove", "kind": "spx", "version": "0.0.1-rc.1.1"})
        sleep.assert_called_once()

    def test_missing_candidate_is_already_removed(self):
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite, "request_spxbench", return_value=(404, "not found")):
            gate = SpxbenchSuiteGate()
            gate.remove_candidate()
        self.assertTrue(gate.candidate_removed)

    def test_success_response_must_confirm_removal(self):
        responses = ({}, {"versions": [{"kind": "spx", "version": "0.0.1-rc.1.1"}]})
        for body in responses:
            with self.subTest(body=body), \
                 patch_candidate_env(), \
                 mock.patch.object(run_spxbench_suite, "request_spxbench", return_value=(200, body)):
                gate = SpxbenchSuiteGate()
                with self.assertRaises(SpxbenchSuiteError):
                    gate.remove_candidate()
                self.assertFalse(gate.candidate_removed)


EVENTS_URL = "http://127.0.0.1/__e2e/api/job-events?job=11"
FINISHED_EVENT = b'event: finished\ndata: {"jobId":11}\n\n'


def event_stream(body=FINISHED_EVENT):
    response = mock.MagicMock()
    response.__enter__.return_value = response
    response.headers.get_content_type.return_value = "text/event-stream"
    response.__iter__.return_value = iter(body.splitlines(keepends=True))
    return response


def unavailable_stream():
    return urllib.error.HTTPError(EVENTS_URL, 503, "unavailable", {}, io.BytesIO(b"deploying"))


@patch_candidate_env()
class SuiteJobEventsTest(unittest.TestCase):
    def test_job_response_rejects_mismatched_id_or_unknown_status(self):
        for job in ({"id": 12, "status": "success"}, {"id": 11}, {"id": 11, "status": "unknown"}):
            with self.subTest(job=job), mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, job)):
                with self.assertRaisesRegex(SpxbenchSuiteError, "state read"):
                    SpxbenchSuiteGate().read_suite_job(11)

    def test_comments_and_multiline_event_data(self):
        stream = io.BytesIO(b": connected\n\n: heartbeat\n\nevent: finished\ndata: {\"jobId\":\ndata: 11}\n\n")
        self.assertEqual(list(sse_events(stream)), [("finished", '{"jobId":\n11}')])

    def test_heartbeats_do_not_extend_overall_deadline(self):
        stream = io.BytesIO(b": connected\n\n: heartbeat\n\n")
        with mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=[99, 99, 101]):
            with self.assertRaisesRegex(SpxbenchSuiteError, "timed out waiting"):
                list(sse_events(stream, deadline=100))

    def test_finished_event_reads_final_state_once(self):
        state = job_response({"running": False, "summary": "passed"})
        response = event_stream(b": connected\n\n" + FINISHED_EVENT)
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", return_value=response) as opened, \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual(opened.call_args.args[0].full_url, EVENTS_URL)
        self.assertEqual(opened.call_args.args[0].get_header("Authorization"), "Bearer t")
        requested.assert_called_once_with("/api/jobs?id=11", timeout=15)

    def test_heartbeats_log_waiting_without_querying_state(self):
        body = b": connected\n\n" + b": heartbeat\n\n" * 5 + FINISHED_EVENT
        response = event_stream(body)
        clock = [0]

        def timed_lines():
            heartbeat_count = 0
            for line in body.splitlines(keepends=True):
                if line == b": heartbeat\n":
                    heartbeat_count += 1
                    clock[0] = heartbeat_count * 15
                yield line

        response.__iter__.return_value = timed_lines()
        state = job_response({"running": False, "summary": "passed"})
        with mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=lambda: clock[0]), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", return_value=response), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested, \
             mock.patch.object(run_spxbench_suite, "log") as logged:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual([call.args[0] for call in logged.call_args_list if "still running" in call.args[0]],
                         ["   job #11 still running; waiting for completion event (60s)"])
        requested.assert_called_once_with("/api/jobs?id=11", timeout=15)

    def test_missing_event_endpoint_fails_with_deployment_error(self):
        error = urllib.error.HTTPError(EVENTS_URL, 404,
                                      "not found", {}, io.BytesIO(b"404 page not found"))
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=error), \
             mock.patch.object(SpxbenchSuiteGate, "request") as requested:
            with self.assertRaisesRegex(SpxbenchSuiteError, "job event stream failed: HTTP 404"):
                SpxbenchSuiteGate().wait_for_suite_job(11)
        requested.assert_not_called()

    def test_disconnect_recovers_from_durable_final_state(self):
        state = job_response({"running": False, "summary": "failed"}, status="failed")
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=urllib.error.URLError("connection lost")) as opened, \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "failed")
        opened.assert_called_once()
        requested.assert_called_once_with("/api/jobs?id=11", timeout=15)

    def test_interrupted_stream_reconnects_while_job_is_running(self):
        def truncated_lines():
            yield b": connected\n"
            yield b"\n"
            yield b"event: finished\n"
            raise http.client.IncompleteRead(b'data: {"jobId":', 3)

        truncated = event_stream(b"")
        truncated.__iter__.return_value = truncated_lines()
        failures = {"disconnect": urllib.error.URLError("lost"), "truncated stream": truncated,
                    "HTTP 503": unavailable_stream()}
        for cause, failure in failures.items():
            states = [(200, job_response({"running": True})),
                      (200, job_response({"running": False, "summary": "passed"}))]
            with self.subTest(cause=cause), \
                 mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                                   side_effect=[failure, event_stream()]) as opened, \
                 mock.patch.object(SpxbenchSuiteGate, "request", side_effect=states) as requested, \
                 mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
                self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
                self.assertEqual(opened.call_count, 2)
                self.assertEqual(requested.call_args_list, [mock.call("/api/jobs?id=11", timeout=15)] * 2)
                sleep.assert_called_once_with(2)

    def test_truncated_forbidden_job_response_fails_without_retry(self):
        body = io.BytesIO()
        error = urllib.error.HTTPError("http://127.0.0.1/api/jobs?id=11", 403, "forbidden", {}, body)
        error.read = mock.Mock(side_effect=http.client.IncompleteRead(b"forb", 5))
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=error) as opened:
            with self.assertRaisesRegex(SpxbenchSuiteError, "state read failed: HTTP 403"):
                SpxbenchSuiteGate().read_suite_job(11)
        opened.assert_called_once()
        self.assertTrue(body.closed)

    def test_incomplete_state_response_reconnects(self):
        truncated = mock.MagicMock()
        truncated.__enter__.return_value = truncated
        truncated.read.side_effect = http.client.IncompleteRead(b'{"jobId":11', 5)
        state_response = mock.MagicMock()
        state_response.__enter__.return_value = state_response
        state_response.status = 200
        state_response.read.return_value = (b'{"id":11,"status":"success","summary":"passed",'
                                             b'"snapshot":{"jobId":11,"running":false,"summary":"passed","cases":[]}}')
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[event_stream(), truncated,
                                            event_stream(), state_response]) as opened, \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual(opened.call_count, 4)
        sleep.assert_called_once_with(2)

    def test_interrupted_503_body_still_checks_durable_state(self):
        for failure in (http.client.IncompleteRead(b"down", 10), TimeoutError("response body timed out"),
                        ConnectionResetError("connection reset by peer")):
            with self.subTest(failure=type(failure).__name__):
                error = unavailable_stream()
                error.read = mock.Mock(side_effect=failure)
                state = job_response({"running": False, "summary": "passed"})
                with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=error), \
                     mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested:
                    self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
                error.read.assert_called_once_with()
                requested.assert_called_once_with("/api/jobs?id=11", timeout=15)

    def test_transient_state_failures_reconnect(self):
        for state_failure in ((503, "deploying"), SpxbenchConnectionError("offline")):
            with self.subTest(state_failure=state_failure):
                response = event_stream()
                state = job_response({"running": False, "summary": "passed"})
                with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                                       side_effect=[urllib.error.URLError("lost"), response]), \
                     mock.patch.object(SpxbenchSuiteGate, "request",
                                       side_effect=[state_failure, (200, state)]) as requested, \
                     mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
                    self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
                self.assertEqual(requested.call_count, 2)
                sleep.assert_called_once_with(2)

    def test_permanent_state_error_does_not_retry(self):
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=urllib.error.URLError("lost")), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(401, "unauthorized")), \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            with self.assertRaisesRegex(SpxbenchSuiteError, "state read failed: HTTP 401"):
                SpxbenchSuiteGate().wait_for_suite_job(11)
        sleep.assert_not_called()

    def test_reconnect_backoff_caps_at_ten_seconds(self):
        response = event_stream()
        state = job_response({"running": False, "summary": "passed"})
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[unavailable_stream() for _ in range(4)] + [response]), \
             mock.patch.object(SpxbenchSuiteGate, "request",
                               side_effect=[(503, "deploying")] * 4 + [(200, state)]), \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [2, 4, 8, 10])

    def test_heartbeat_resets_reconnect_backoff(self):
        streams = [unavailable_stream(), unavailable_stream(), event_stream(b": heartbeat\n\n"),
                   unavailable_stream(), event_stream()]
        running = (200, job_response({"running": True}))
        passed = (200, job_response({"running": False, "summary": "passed"}))
        with mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=streams), \
             mock.patch.object(SpxbenchSuiteGate, "request",
                               side_effect=[running] * 4 + [passed]), \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [2, 4, 2, 4])

    def test_unavailable_stream_stops_after_recovery_budget(self):
        clock = [0]
        stream_attempts = []
        state_attempts = []

        def unavailable(*args, **kwargs):
            stream_attempts.append((clock[0], kwargs["timeout"]))
            raise unavailable_stream()

        def unavailable_state(path, **kwargs):
            state_attempts.append((clock[0], kwargs["timeout"]))
            return 503, "deploying"

        with mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=lambda: clock[0]), \
             mock.patch.object(run_spxbench_suite.time, "sleep", side_effect=lambda seconds: clock.__setitem__(0, clock[0] + seconds)), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=unavailable), \
             mock.patch.object(SpxbenchSuiteGate, "request", side_effect=unavailable_state), \
             mock.patch.object(run_spxbench_suite, "log"):
            with self.assertRaisesRegex(SpxbenchSuiteError, "event stream unavailable for 120 seconds"):
                SpxbenchSuiteGate().wait_for_suite_job(11)
        self.assertEqual(clock[0], 120)
        self.assertTrue(stream_attempts)
        self.assertTrue(state_attempts)
        self.assertTrue(all(attempt_time < 120 for attempt_time, _ in stream_attempts + state_attempts))
        self.assertLessEqual(stream_attempts[-1][1], 10)
        self.assertLessEqual(state_attempts[-1][1], 10)


class TimeoutTest(unittest.TestCase):
    def test_invalid_timeout_is_reported_as_configuration_error(self):
        with patch_candidate_env(SPXBENCH_TIMEOUT_MINUTES="zero"):
            with self.assertRaisesRegex(SpxbenchSuiteError, "positive integer"):
                SpxbenchSuiteGate()

    def test_busy_retry_stops_at_deadline(self):
        with patch_candidate_env(), \
             mock.patch.object(SpxbenchSuiteGate, "request",
                               return_value=(409, {"code": "run_slot_busy"})):
            gate = SpxbenchSuiteGate()
            gate.deadline = 101
            with mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=(100, 101)), \
                 mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
                with self.assertRaisesRegex(SpxbenchSuiteError, "timed out waiting"):
                    gate.start_suite_run(7)
            sleep.assert_called_once_with(1)

    def test_api_request_uses_remaining_budget(self):
        with patch_candidate_env():
            gate = SpxbenchSuiteGate()
            gate.deadline = 105
            with mock.patch.object(run_spxbench_suite.time, "monotonic", return_value=100), \
                 mock.patch.object(run_spxbench_suite, "request_spxbench", return_value=(200, {})) as requested:
                gate.request("/api/suites")
                self.assertEqual(requested.call_args.kwargs["timeout"], 5)
                gate.deadline = 100
                with self.assertRaisesRegex(SpxbenchSuiteError, "exceeded"):
                    gate.request("/api/suites")
                requested.assert_called_once()


class CandidateLifecycleTest(unittest.TestCase):
    def setUp(self):
        stack = ExitStack()
        self.addCleanup(stack.close)
        directory = Path(stack.enter_context(tempfile.TemporaryDirectory()))
        self.archive = directory / "spx_web.zip"
        self.archive.write_bytes(ARCHIVE)
        self.summary = directory / "summary"
        stack.enter_context(patch_candidate_env(
            WEB_ARCHIVE_PATH=str(self.archive), GITHUB_STEP_SUMMARY=str(self.summary)))
        self.removed = stack.enter_context(mock.patch.object(
            run_spxbench_suite, "request_spxbench", return_value=(200, {"versions": []})))
        self.request = stack.enter_context(mock.patch.object(SpxbenchSuiteGate, "request"))
        stack.enter_context(mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                                             return_value=event_stream()))

    def run_suite(self, *, passed=True):
        job = job_response({
            "running": False, "total": 1, "completed": 1, "passed": int(passed),
            "summary": "passed" if passed else "failed",
            "cases": [{"name": "smoke", "status": "pass" if passed else "fail"}],
        }, status="success" if passed else "failed")
        self.request.side_effect = [SUITE_LIST, INSTALL_ACCEPTED, INSTALL_SUCCEEDED, CANDIDATE_REGISTERED,
                                    (200, {"jobId": 11}), (200, job)]
        run_spxbench_suite.main()

    def assert_report(self, *, regression, runtime):
        report = self.summary.read_text()
        self.assertIn(f"Regression: **{regression}**", report)
        self.assertIn(f"spxbench candidate: **{runtime}**", report)
        self.assertIn("[123](https://github.com/goplus/spx/actions/runs/42/artifacts/123)", report)

    def test_candidate_is_pinned_across_install_and_run(self):
        self.run_suite()
        self.assert_report(regression="passed", runtime="removed")
        self.assertIn("/__e2e?job=11", self.summary.read_text())
        self.removed.assert_called_once_with("http://127.0.0.1/api/runtime-versions", "t", method="POST",
                                            payload={"action": "remove", "kind": "spx", "version": VERSION},
                                            timeout=mock.ANY)
        self.assertEqual(self.request.call_args_list[1], INSTALL_REQUEST)
        self.assertEqual(self.request.call_args_list[4],
                         mock.call(RUN_PATH, method="PUT"))

    def test_pr_candidate_uses_tested_merge_instead_of_workflow_commit(self):
        with mock.patch.dict(os.environ, {"CANDIDATE_SHA": COMMIT, "GITHUB_SHA": "f" * 40}):
            self.run_suite()
        self.assertEqual(self.request.call_args_list[1], INSTALL_REQUEST)

    def test_install_failure_skips_candidate_removal(self):
        self.request.side_effect = [SUITE_LIST, (403, "forbidden")]
        with self.assertRaisesRegex(SpxbenchSuiteError, "install request failed: HTTP 403"):
            run_spxbench_suite.main()
        self.assertEqual(self.request.call_args_list, [mock.call("/api/suites"), INSTALL_REQUEST])
        self.assert_report(regression="not completed", runtime="retained")
        self.removed.assert_not_called()

    def test_failed_suite_retains_runtime(self):
        with self.assertRaisesRegex(SpxbenchSuiteError, "failed suite"):
            self.run_suite(passed=False)
        self.assert_report(regression="failed", runtime="retained")
        self.removed.assert_not_called()

    def test_remove_failure_warns_without_failing_suite(self):
        self.removed.return_value = (500, "temporarily\nunavailable")
        self.run_suite()
        self.assert_report(regression="passed", runtime="failed")
        self.assertIn("Cleanup warning: spxbench candidate removal: remove candidate failed: HTTP 500: "
                      "temporarily unavailable", self.summary.read_text())


if __name__ == "__main__":
    unittest.main()
