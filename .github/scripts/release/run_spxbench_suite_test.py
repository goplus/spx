"""Unit tests for the spxbench release suite gate."""

import hashlib
import http.client
import io
import os
import tempfile
import unittest
import urllib.error
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
    "GITHUB_REPOSITORY": "goplus/spx", "GITHUB_SHA": COMMIT,
}
OPERATION_ID = f"spx:{VERSION}"
INSTALL_ACCEPTED = (202, {"operationId": OPERATION_ID, "status": "running"})
INSTALL_SUCCEEDED = (200, {"operationId": OPERATION_ID, "status": "success"})
ARCHIVE = b"candidate Web archive"
DIGEST = "sha256:" + hashlib.sha256(ARCHIVE).hexdigest()
CANDIDATE_REGISTERED = (200, {"versions": [{"kind": "spx", "version": VERSION, "sourceCommit": COMMIT, "digest": DIGEST}]})
RUN_PATH = f"/__e2e/api/run?suite=7&spxVersion={VERSION}&trigger=spx-dry-run&repository=goplus%2Fspx"
INSTALL_REQUEST = mock.call(
    "/api/runtime-versions/install", method="POST", timeout=30,
    payload={"kind": "spx", "version": VERSION, "sourceCommit": COMMIT,
             "sourceUrl": f"https://github.com/goplus/spx/releases/download/v{VERSION}/spx_web.zip"})
OPERATION_REQUEST = mock.call(f"/api/runtime-versions/operations/spx%3A{VERSION}", timeout=30)
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

    def test_already_completed_install_is_accepted(self):
        self.request.return_value = CANDIDATE_REGISTERED
        SpxbenchSuiteGate().install_candidate(COMMIT)
        self.assertEqual(self.request.call_args_list, [INSTALL_REQUEST])

    def test_registered_candidate_with_different_commit_is_rejected(self):
        self.request.return_value = (200, {"versions": [{"kind": "spx", "version": VERSION, "sourceCommit": "f" * 40}]})
        with self.assertRaisesRegex(SpxbenchSuiteError, "does not match"):
            SpxbenchSuiteGate().install_candidate(COMMIT)

    def test_missing_or_mismatched_archive_digest_is_rejected(self):
        for digest in (None, "sha256:other"):
            self.request.return_value = (200, {"versions": [{"kind": "spx", "version": VERSION,
                                                           "sourceCommit": COMMIT, "digest": digest}]})
            with self.subTest(digest=digest), self.assertRaisesRegex(SpxbenchSuiteError, "archive digest"):
                SpxbenchSuiteGate().install_candidate(COMMIT)

    def test_invalid_install_response_is_rejected_without_polling(self):
        for response, message in (
            ((200, {"versions": []}), "did not include candidate"),
            ((202, {"status": "running"}), "missing operationId"),
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


class DeleteCandidateReleaseTest(unittest.TestCase):
    def test_command_results_preserve_failure_detail_and_logs(self):
        cases = (
            (0, "ignored", "ignored", None),
            (1, " denied ", "fallback", "denied"),
            (2, "", " missing ", "missing"),
            (3, "", "", "gh exited 3"),
            (4, "   ", "ignored", "gh exited 4"),
        )
        for code, stderr, stdout, failure in cases:
            with self.subTest(code=code), \
                 mock.patch.object(run_spxbench_suite, "run_command",
                                   return_value=mock.Mock(returncode=code, stderr=stderr, stdout=stdout)) as command, \
                 mock.patch.object(run_spxbench_suite, "log") as log:
                self.assertEqual(run_spxbench_suite.delete_candidate_release("vtest"), failure)
                command.assert_called_once_with("gh", "release", "delete", "vtest",
                                                "--cleanup-tag", "--yes", check=False)
                expected = [mock.call("deleting candidate prerelease vtest")]
                if failure is not None:
                    expected.append(mock.call(f"candidate prerelease deletion failed: {failure}"))
                self.assertEqual(log.call_args_list, expected)

    def test_os_error_is_reported_without_raising(self):
        for message in ("gh unavailable", ""):
            with self.subTest(message=message), \
                 mock.patch.object(run_spxbench_suite, "run_command", side_effect=OSError(message)), \
                 mock.patch.object(run_spxbench_suite, "log") as log:
                self.assertEqual(run_spxbench_suite.delete_candidate_release("vtest"), message)
                self.assertEqual(log.call_args_list, [
                    mock.call("deleting candidate prerelease vtest"),
                    mock.call(f"candidate prerelease deletion failed: {message}"),
                ])


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


class CandidateReleaseCleanupTest(unittest.TestCase):
    def setUp(self):
        stack = ExitStack()
        self.addCleanup(stack.close)
        directory = Path(stack.enter_context(tempfile.TemporaryDirectory()))
        self.archive = directory / "spx_web.zip"
        self.archive.write_bytes(ARCHIVE)
        self.output = directory / "outputs"
        self.summary = directory / "summary"
        stack.enter_context(patch_candidate_env(
            WEB_ARCHIVE_PATH=str(self.archive), GITHUB_OUTPUT=str(self.output), GITHUB_STEP_SUMMARY=str(self.summary)))
        self.created = stack.enter_context(mock.patch.object(run_spxbench_suite, "create_candidate_release"))
        self.deleted = stack.enter_context(mock.patch.object(
            run_spxbench_suite, "delete_candidate_release", return_value=None))
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
        self.request.side_effect = [(200, {"suites": [{"id": 7, "name": "spx"}]}),
                                    INSTALL_ACCEPTED, INSTALL_SUCCEEDED, CANDIDATE_REGISTERED,
                                    (200, {"jobId": 11}), (200, job)]
        run_spxbench_suite.main()

    def assert_cleanup_states(self, *states):
        self.assertEqual(self.output.read_text().splitlines(),
                         [f"prerelease_cleanup_pending={str(state).lower()}" for state in states])

    def assert_report(self, *, regression, runtime, prerelease):
        report = self.summary.read_text()
        self.assertIn(f"Regression: **{regression}**", report)
        self.assertIn(f"spxbench candidate: **{runtime}**", report)
        self.assertIn(f"GitHub prerelease: **{prerelease}**", report)

    def test_creation_failure_still_cleans_up_and_preserves_original_error(self):
        self.created.side_effect = SpxbenchSuiteError("release creation failed")
        for deletion_error, states in ((None, [True, False]), ("release not found", [True])):
            with self.subTest(deletion_error=deletion_error):
                self.output.write_text("")
                self.deleted.reset_mock()
                self.deleted.return_value = deletion_error
                with self.assertRaisesRegex(SpxbenchSuiteError, "release creation failed"):
                    run_spxbench_suite.main()
                self.deleted.assert_called_once_with(f"v{VERSION}")
                self.assert_cleanup_states(*states)

    def test_prerelease_cleanup_failure_does_not_fail_passing_suite(self):
        self.deleted.return_value = "permission denied"
        self.run_suite()
        self.assert_cleanup_states(True, False, True)
        self.assert_report(regression="passed", runtime="removed", prerelease="failed")
        self.assertIn("permission denied", self.summary.read_text())

    def test_candidate_is_pinned_across_release_install_and_run(self):
        def remove(*args, **kwargs):
            self.assert_cleanup_states(True, False)
            return 200, {"versions": []}

        self.created.side_effect = lambda *args: self.assert_cleanup_states(True)
        self.removed.side_effect = remove
        self.deleted.side_effect = lambda tag: self.assert_cleanup_states(True, False, True)
        self.run_suite()
        self.assert_cleanup_states(True, False, True, False)
        self.assert_report(regression="passed", runtime="removed", prerelease="deleted")
        self.assertIn("/__e2e?job=11", self.summary.read_text())
        self.created.assert_called_once_with(f"v{VERSION}", str(self.archive), COMMIT)
        self.deleted.assert_called_once_with(f"v{VERSION}")
        self.removed.assert_called_once_with("http://127.0.0.1/api/runtime-versions", "t", method="POST",
                                            payload={"action": "remove", "kind": "spx", "version": VERSION},
                                            timeout=mock.ANY)
        self.assertEqual(self.request.call_args_list[1], INSTALL_REQUEST)
        self.assertEqual(self.request.call_args_list[4],
                         mock.call(RUN_PATH, method="PUT"))

    def test_pr_candidate_uses_tested_merge_instead_of_workflow_commit(self):
        with mock.patch.dict(os.environ, {"CANDIDATE_SHA": COMMIT, "GITHUB_SHA": "f" * 40}):
            self.run_suite()
        self.created.assert_called_once_with(f"v{VERSION}", str(self.archive), COMMIT)
        self.assertEqual(self.request.call_args_list[1], INSTALL_REQUEST)

    def test_install_failure_retains_candidate_and_prerelease(self):
        def request(path, **kwargs):
            if path == "/api/suites":
                return 200, {"suites": [{"id": 7, "name": "spx"}]}
            self.assertEqual(path, "/api/runtime-versions/install")
            self.assert_cleanup_states(True, False)
            return 403, "forbidden"

        self.request.side_effect = request
        with self.assertRaisesRegex(SpxbenchSuiteError, "install request failed: HTTP 403"):
            run_spxbench_suite.main()
        self.assert_cleanup_states(True, False)
        self.assert_report(regression="not completed", runtime="retained", prerelease="retained")
        self.removed.assert_not_called()
        self.deleted.assert_not_called()

    def test_failed_suite_retains_runtime_and_prerelease(self):
        with self.assertRaisesRegex(SpxbenchSuiteError, "failed suite"):
            self.run_suite(passed=False)
        self.assert_cleanup_states(True, False)
        self.assert_report(regression="failed", runtime="retained", prerelease="retained")
        self.removed.assert_not_called()
        self.deleted.assert_not_called()

    def test_remove_failure_retains_prerelease(self):
        self.removed.return_value = (500, "unavailable")
        self.run_suite()
        self.assert_cleanup_states(True, False)
        self.assert_report(regression="passed", runtime="failed", prerelease="retained")
        self.assertIn("HTTP 500", self.summary.read_text())
        self.deleted.assert_not_called()


if __name__ == "__main__":
    unittest.main()
