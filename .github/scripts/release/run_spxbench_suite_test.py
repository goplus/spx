"""Unit tests for the spxbench release suite gate."""

import http.client
import io
import os
import tempfile
import unittest
import urllib.error
from unittest import mock

import run_spxbench_suite
from run_spxbench_suite import (SpxbenchConnectionError, SpxbenchSuiteError, SpxbenchSuiteGate, failing_cases,
                                find_suite_id, is_run_slot_busy, job_failure_reasons, require_env, sse_events)


CANDIDATE_ENV = {"SPXBENCH_URL": "http://127.0.0.1", "SPXBENCH_API_TOKEN": "t",
                 "WEB_ARCHIVE_PATH": "dist/spx_web.zip", "CANDIDATE_VERSION": "0.0.1-rc.1.1", "GITHUB_SHA": "abc123"}


def patch_candidate_env(**updates):
    return mock.patch.dict(os.environ, {**CANDIDATE_ENV, **updates}, clear=True)


class FindSuiteIdTest(unittest.TestCase):
    def test_matches_by_exact_name(self):
        suites = [{"id": 7, "name": "spx"}, {"id": 9, "name": "smoke"}]
        self.assertEqual(find_suite_id(suites, "spx"), 7)
        self.assertEqual(find_suite_id(suites, "smoke"), 9)

    def test_missing_name_returns_none(self):
        self.assertIsNone(find_suite_id([{"id": 1, "name": "spx"}], "ghost"))
        self.assertIsNone(find_suite_id(None, "spx"))
        self.assertIsNone(find_suite_id([], "spx"))


class FailingCasesTest(unittest.TestCase):
    def test_collects_non_pass_cases(self):
        state = {"cases": [{"name": "a", "status": "pass"},
                           {"name": "b", "status": "fail"},
                           {"name": "c", "status": "skip"}]}
        self.assertEqual([case["name"] for case in failing_cases(state)], ["b", "c"])

    def test_empty_state_has_no_failures(self):
        self.assertEqual(failing_cases({}), [])
        self.assertEqual(failing_cases({"cases": None}), [])


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


class JobVerdictTest(unittest.TestCase):
    def test_all_cases_pass_but_job_verdict_failed_is_red(self):
        state = {"summary": "failed", "lastError": "runner crashed", "total": 2, "completed": 2,
                 "cases": [{"name": "a", "status": "pass"}, {"name": "b", "status": "pass"}]}
        reasons = job_failure_reasons(state)
        self.assertTrue(any("summary" in reason for reason in reasons))
        self.assertTrue(any("job error" in reason for reason in reasons))

    def test_clean_success_has_no_failure_reasons(self):
        state = {"summary": "passed", "total": 2, "completed": 2, "passed": 2,
                 "cases": [{"name": "a", "status": "pass"}, {"name": "b", "status": "pass"}]}
        self.assertEqual(job_failure_reasons(state), [])

    def test_empty_suite_is_red_even_if_summary_passed(self):
        state = {"summary": "passed", "total": 0, "completed": 0, "passed": 0, "cases": []}
        self.assertIn("job selected no test cases", job_failure_reasons(state))

    def test_passed_summary_with_unpassed_case_is_red(self):
        state = {"summary": "passed", "total": 2, "completed": 2, "passed": 1,
                 "cases": [{"name": "a", "status": "pass"}]}
        self.assertIn("job passed 1 of 2 cases", job_failure_reasons(state))

    def test_incomplete_job_is_red(self):
        state = {"summary": "passed", "total": 5, "completed": 4,
                 "cases": [{"name": "a", "status": "pass"}]}
        self.assertTrue(any("incomplete" in reason for reason in job_failure_reasons(state)))

    def test_failed_case_list_is_reported(self):
        state = {"summary": "passed", "total": 2, "completed": 2,
                 "cases": [{"name": "a", "status": "pass"}, {"name": "b", "status": "fail"}]}
        self.assertTrue(any("did not pass" in reason for reason in job_failure_reasons(state)))


class SuiteRunConflictTest(unittest.TestCase):
    def test_busy_conflict_is_recoverable(self):
        self.assertTrue(is_run_slot_busy("a test run is already in progress\n"))

    def test_permanent_rejection_is_not_busy(self):
        self.assertFalse(is_run_slot_busy("suite 1 selects no ready test cases\n"))
        self.assertFalse(is_run_slot_busy({"error": "boom"}))

    def test_success_without_job_id_fails_immediately(self):
        with patch_candidate_env(), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, {})):
            gate = SpxbenchSuiteGate()
            with self.assertRaisesRegex(SpxbenchSuiteError, "missing jobId"):
                gate.start_suite_run(7)

    def test_permanent_conflicts_and_http_errors_do_not_retry(self):
        cases = ((409, " no ready cases ", "run rejected: no ready cases"),
                 (409, {"error": "boom"}, 'run rejected: {"error": "boom"}'),
                 (500, "unavailable", "run failed: HTTP 500: unavailable"))
        for status, body, message in cases:
            with self.subTest(status=status, body=body), \
                 patch_candidate_env(), \
                 mock.patch.object(SpxbenchSuiteGate, "request", return_value=(status, body)) as requested, \
                 mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
                with self.assertRaises(SpxbenchSuiteError) as caught:
                    SpxbenchSuiteGate().start_suite_run(7)
                self.assertEqual(str(caught.exception), message)
                requested.assert_called_once_with("/__e2e/api/run?suite=7&spxVersion=0.0.1-rc.1.1", method="POST")
                sleep.assert_not_called()


class CandidateInstallTest(unittest.TestCase):
    def test_install_response_must_include_candidate(self):
        with patch_candidate_env(GITHUB_REPOSITORY="goplus/spx"), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, {"versions": []})):
            gate = SpxbenchSuiteGate()
            with self.assertRaisesRegex(SpxbenchSuiteError, "did not include candidate"):
                gate.install_candidate()


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


class SuiteJobEventsTest(unittest.TestCase):
    @staticmethod
    def stream(body):
        response = mock.MagicMock()
        response.__enter__.return_value = response
        response.headers.get_content_type.return_value = "text/event-stream"
        response.__iter__.return_value = iter(body.splitlines(keepends=True))
        return response

    def test_comments_and_multiline_event_data(self):
        stream = io.BytesIO(b": connected\n\n: heartbeat\n\nevent: finished\ndata: {\"jobId\":\ndata: 11}\n\n")
        self.assertEqual(list(sse_events(stream)), [("finished", '{"jobId":\n11}')])

    def test_heartbeats_do_not_extend_overall_deadline(self):
        stream = io.BytesIO(b": connected\n\n: heartbeat\n\n")
        with mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=[99, 99, 101]):
            with self.assertRaisesRegex(SpxbenchSuiteError, "timed out waiting"):
                list(sse_events(stream, deadline=100))

    def test_finished_event_reads_final_state_once(self):
        state = {"jobId": 11, "running": False, "summary": "passed"}
        response = self.stream(b": connected\n\nevent: finished\ndata: {\"jobId\":11}\n\n")
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", return_value=response) as opened, \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested:
            self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), state)
        self.assertEqual(opened.call_args.args[0].full_url,
                         "http://127.0.0.1/__e2e/api/job-events?job=11")
        self.assertEqual(opened.call_args.args[0].get_header("Authorization"), "Bearer t")
        requested.assert_called_once_with("/__e2e/api/state?job=11", timeout=15)

    def test_heartbeats_log_waiting_without_querying_state(self):
        body = (b": connected\n\n" + b": heartbeat\n\n" * 5
                + b"event: finished\ndata: {\"jobId\":11}\n\n")
        response = self.stream(body)
        clock = [0]

        def timed_lines():
            heartbeat_count = 0
            for line in body.splitlines(keepends=True):
                if line == b": heartbeat\n":
                    heartbeat_count += 1
                    clock[0] = heartbeat_count * 15
                yield line

        response.__iter__.return_value = timed_lines()
        state = {"jobId": 11, "running": False, "summary": "passed"}
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=lambda: clock[0]), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", return_value=response), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested, \
             mock.patch.object(run_spxbench_suite, "log") as logged:
            self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), state)
        self.assertEqual([call.args[0] for call in logged.call_args_list if "still running" in call.args[0]],
                         ["   job #11 still running; waiting for completion event (60s)"])
        requested.assert_called_once_with("/__e2e/api/state?job=11", timeout=15)

    def test_missing_event_endpoint_fails_with_deployment_error(self):
        error = urllib.error.HTTPError("http://127.0.0.1/__e2e/api/job-events?job=11", 404,
                                      "not found", {}, io.BytesIO(b"404 page not found"))
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=error), \
             mock.patch.object(SpxbenchSuiteGate, "request") as requested:
            with self.assertRaisesRegex(SpxbenchSuiteError, "job event stream failed: HTTP 404"):
                SpxbenchSuiteGate().wait_for_suite_job(11)
        requested.assert_not_called()

    def test_disconnect_recovers_from_durable_final_state(self):
        state = {"jobId": 11, "running": False, "summary": "failed"}
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=urllib.error.URLError("connection lost")) as opened, \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested:
            self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), state)
        opened.assert_called_once()
        requested.assert_called_once_with("/__e2e/api/state?job=11", timeout=15)

    def test_disconnect_while_running_reconnects(self):
        response = self.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")
        states = [(200, {"jobId": 11, "running": True}),
                  (200, {"jobId": 11, "running": False, "summary": "passed"})]
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[urllib.error.URLError("lost"), response]) as opened, \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep, \
             mock.patch.object(SpxbenchSuiteGate, "request", side_effect=states) as requested:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual(opened.call_count, 2)
        self.assertEqual(requested.call_count, 2)
        sleep.assert_called_once()

    def test_incomplete_event_stream_checks_state_and_reconnects(self):
        def truncated_lines():
            yield b": connected\n"
            yield b"\n"
            yield b"event: finished\n"
            raise http.client.IncompleteRead(b'data: {"jobId":', 3)

        truncated = self.stream(b"")
        truncated.__iter__.return_value = truncated_lines()
        finished = self.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")
        states = [(200, {"jobId": 11, "running": True}),
                  (200, {"jobId": 11, "running": False, "summary": "passed"})]
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[truncated, finished]) as opened, \
             mock.patch.object(SpxbenchSuiteGate, "request", side_effect=states) as requested, \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual(opened.call_count, 2)
        self.assertEqual(requested.call_count, 2)
        sleep.assert_called_once_with(2)

    def test_incomplete_state_response_reconnects(self):
        truncated = mock.MagicMock()
        truncated.__enter__.return_value = truncated
        truncated.read.side_effect = http.client.IncompleteRead(b'{"jobId":11', 5)
        finished_event = b"event: finished\ndata: {\"jobId\":11}\n\n"
        state_response = mock.MagicMock()
        state_response.__enter__.return_value = state_response
        state_response.status = 200
        state_response.read.return_value = b'{"jobId":11,"running":false,"summary":"passed"}'
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[self.stream(finished_event), truncated,
                                            self.stream(finished_event), state_response]) as opened, \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual(opened.call_count, 4)
        sleep.assert_called_once_with(2)

    def test_stream_503_recovers_after_state_check(self):
        error = urllib.error.HTTPError("http://127.0.0.1/__e2e/api/job-events?job=11", 503,
                                      "unavailable", {}, io.BytesIO(b"deploying"))
        response = self.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")
        states = [(200, {"jobId": 11, "running": True}),
                  (200, {"jobId": 11, "running": False, "summary": "passed"})]
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[error, response]) as opened, \
             mock.patch.object(SpxbenchSuiteGate, "request", side_effect=states) as requested, \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual(opened.call_count, 2)
        self.assertEqual(requested.call_count, 2)
        sleep.assert_called_once_with(2)

    def test_truncated_503_body_still_checks_durable_state(self):
        error = urllib.error.HTTPError("http://127.0.0.1/__e2e/api/job-events?job=11", 503,
                                      "unavailable", {}, io.BytesIO())
        error.read = mock.Mock(side_effect=http.client.IncompleteRead(b"down", 10))
        state = {"jobId": 11, "running": False, "summary": "passed"}
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=error), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested:
            self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), state)
        error.read.assert_called_once_with()
        requested.assert_called_once_with("/__e2e/api/state?job=11", timeout=15)

    def test_timed_out_503_body_still_checks_durable_state(self):
        error = urllib.error.HTTPError("http://127.0.0.1/__e2e/api/job-events?job=11", 503,
                                      "unavailable", {}, io.BytesIO())
        error.read = mock.Mock(side_effect=TimeoutError("response body timed out"))
        state = {"jobId": 11, "running": False, "summary": "passed"}
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen", side_effect=error), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, state)) as requested:
            self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), state)
        error.read.assert_called_once_with()
        requested.assert_called_once_with("/__e2e/api/state?job=11", timeout=15)

    def test_transient_state_failures_reconnect(self):
        for state_failure in ((503, "deploying"), SpxbenchConnectionError("offline")):
            with self.subTest(state_failure=state_failure):
                response = self.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")
                state = {"jobId": 11, "running": False, "summary": "passed"}
                with patch_candidate_env(), \
                     mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                                       side_effect=[urllib.error.URLError("lost"), response]), \
                     mock.patch.object(SpxbenchSuiteGate, "request",
                                       side_effect=[state_failure, (200, state)]) as requested, \
                     mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
                    self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), state)
                self.assertEqual(requested.call_count, 2)
                sleep.assert_called_once_with(2)

    def test_permanent_state_error_does_not_retry(self):
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=urllib.error.URLError("lost")), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(401, "unauthorized")), \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            with self.assertRaisesRegex(SpxbenchSuiteError, "state read failed: HTTP 401"):
                SpxbenchSuiteGate().wait_for_suite_job(11)
        sleep.assert_not_called()

    def test_reconnect_backoff_caps_at_ten_seconds(self):
        def unavailable():
            return urllib.error.HTTPError("http://127.0.0.1/__e2e/api/job-events?job=11", 503,
                                          "unavailable", {}, io.BytesIO(b"deploying"))

        response = self.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")
        state = {"jobId": 11, "running": False, "summary": "passed"}
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[unavailable() for _ in range(4)] + [response]), \
             mock.patch.object(SpxbenchSuiteGate, "request",
                               side_effect=[(503, "deploying")] * 4 + [(200, state)]), \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), state)
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [2, 4, 8, 10])

    def test_heartbeat_resets_reconnect_backoff(self):
        def unavailable():
            return urllib.error.HTTPError("http://127.0.0.1/__e2e/api/job-events?job=11", 503,
                                          "unavailable", {}, io.BytesIO(b"deploying"))

        heartbeat = self.stream(b": heartbeat\n\n")
        finished = self.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")
        running = (200, {"jobId": 11, "running": True})
        passed = (200, {"jobId": 11, "running": False, "summary": "passed"})
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                               side_effect=[unavailable(), unavailable(), heartbeat, unavailable(), finished]), \
             mock.patch.object(SpxbenchSuiteGate, "request",
                               side_effect=[running, running, running, running, passed]), \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
            self.assertEqual(SpxbenchSuiteGate().wait_for_suite_job(11)["summary"], "passed")
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [2, 4, 2, 4])

    def test_unavailable_stream_stops_after_recovery_budget(self):
        clock = [0]
        stream_attempts = []
        state_attempts = []

        def unavailable(*args, **kwargs):
            stream_attempts.append((clock[0], kwargs["timeout"]))
            raise urllib.error.HTTPError("http://127.0.0.1/__e2e/api/job-events?job=11", 503,
                                         "unavailable", {}, io.BytesIO(b"deploying"))

        def unavailable_state(path, **kwargs):
            state_attempts.append((clock[0], kwargs["timeout"]))
            return 503, "deploying"

        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=lambda: clock[0]), \
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
                               return_value=(409, "a test run is already in progress")):
            gate = SpxbenchSuiteGate()
            gate.deadline = 101
            with mock.patch.object(run_spxbench_suite.time, "monotonic", side_effect=(100, 100, 101)), \
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
    def test_creation_failure_still_cleans_up(self):
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite, "create_candidate_release",
                               side_effect=SpxbenchSuiteError("boom")), \
             mock.patch.object(run_spxbench_suite, "delete_candidate_release", return_value=None) as deleted:
            with self.assertRaises(SpxbenchSuiteError):
                run_spxbench_suite.main()
            deleted.assert_called_once_with("v0.0.1-rc.1.1")

    def test_cleanup_failure_does_not_hide_creation_failure(self):
        with patch_candidate_env(), \
             mock.patch.object(run_spxbench_suite, "create_candidate_release",
                               side_effect=SpxbenchSuiteError("release creation failed")), \
             mock.patch.object(run_spxbench_suite, "delete_candidate_release", return_value="release not found"):
            with self.assertRaisesRegex(SpxbenchSuiteError, "release creation failed"):
                run_spxbench_suite.main()

    def test_successful_validation_reports_prerelease_cleanup_failure_without_failing_gate(self):
        with tempfile.TemporaryDirectory() as directory:
            output_path = os.path.join(directory, "outputs")
            summary_path = os.path.join(directory, "summary")
            with patch_candidate_env(GITHUB_OUTPUT=output_path, GITHUB_STEP_SUMMARY=summary_path), \
                 mock.patch.object(run_spxbench_suite, "create_candidate_release"), \
                 mock.patch.object(run_spxbench_suite, "delete_candidate_release", return_value="permission denied"), \
                 mock.patch.object(run_spxbench_suite, "request_spxbench", return_value=(200, {"versions": []})), \
                 mock.patch.object(SpxbenchSuiteGate, "resolve_suite", return_value=7), \
                 mock.patch.object(SpxbenchSuiteGate, "install_candidate"), \
                 mock.patch.object(SpxbenchSuiteGate, "start_suite_run", return_value=1), \
                 mock.patch.object(SpxbenchSuiteGate, "wait_for_suite_job", return_value={"summary": "passed", "total": 1,
                                                                                     "completed": 1, "passed": 1, "cases": []}):
                run_spxbench_suite.main()
            with open(output_path) as output:
                self.assertEqual(output.read().splitlines(),
                                 ["candidate_removed=true", "candidate_release_cleanup_failed=true"])
            with open(summary_path) as summary:
                report = summary.read()
            self.assertIn("Regression: **passed**", report)
            self.assertIn("spxbench candidate: **removed**", report)
            self.assertIn("GitHub prerelease: **failed**", report)
            self.assertIn("permission denied", report)

    def test_candidate_is_pinned_across_release_install_and_run(self):
        responses = [(200, {"suites": [{"id": 7, "name": "spx"}]}),
                     (200, {"versions": [{"kind": "spx", "version": "0.0.1-rc.1.1", "digest": "abc"}]}),
                     (200, {"jobId": 11}),
                     (200, {"jobId": 11, "running": False, "summary": "passed", "total": 1,
                            "completed": 1, "passed": 1, "cases": [{"name": "smoke", "status": "pass"}]})]
        with tempfile.TemporaryDirectory() as directory:
            output_path = os.path.join(directory, "outputs")
            summary_path = os.path.join(directory, "summary")
            events = []
            with patch_candidate_env(GITHUB_REPOSITORY="goplus/spx", GITHUB_SHA="abc123",
                                     GITHUB_OUTPUT=output_path, GITHUB_STEP_SUMMARY=summary_path), \
                 mock.patch.object(run_spxbench_suite, "create_candidate_release") as created, \
                 mock.patch.object(run_spxbench_suite, "delete_candidate_release",
                                   side_effect=lambda tag: events.append("release") or None) as deleted, \
                 mock.patch.object(run_spxbench_suite, "request_spxbench",
                                   side_effect=lambda *args, **kwargs: events.append("runtime") or
                                   (200, {"versions": []})) as removed, \
                 mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                                   return_value=SuiteJobEventsTest.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")), \
                 mock.patch.object(SpxbenchSuiteGate, "request", side_effect=responses) as requested:
                run_spxbench_suite.main()
            with open(output_path) as output:
                self.assertEqual(output.read().splitlines(),
                                 ["candidate_install_started=true", "candidate_removed=true"])
            with open(summary_path) as summary:
                report = summary.read()
            self.assertIn("Regression: **passed**", report)
            self.assertIn("spxbench candidate: **removed**", report)
            self.assertIn("GitHub prerelease: **deleted**", report)
            self.assertIn("/__e2e?job=11", report)
        created.assert_called_once_with("v0.0.1-rc.1.1", "dist/spx_web.zip", "abc123")
        deleted.assert_called_once_with("v0.0.1-rc.1.1")
        self.assertEqual(events, ["runtime", "release"])
        removed.assert_called_once_with("http://127.0.0.1/api/runtime-versions", "t", method="POST",
                                        payload={"action": "remove", "kind": "spx", "version": "0.0.1-rc.1.1"},
                                        timeout=mock.ANY)
        self.assertEqual(requested.call_args_list[1], mock.call("/api/runtime-versions", method="POST",
            payload={"action": "install", "kind": "spx", "version": "0.0.1-rc.1.1",
                     "sourceUrl": "https://github.com/goplus/spx/releases/download/v0.0.1-rc.1.1/spx_web.zip"}))
        self.assertEqual(requested.call_args_list[2],
                         mock.call("/__e2e/api/run?suite=7&spxVersion=0.0.1-rc.1.1", method="POST"))

    def test_failed_suite_retains_runtime_and_prerelease(self):
        responses = [(200, {"suites": [{"id": 7, "name": "spx"}]}),
                     (200, {"versions": [{"kind": "spx", "version": "0.0.1-rc.1.1"}]}),
                     (200, {"jobId": 11}),
                     (200, {"jobId": 11, "running": False, "summary": "failed", "total": 1,
                            "completed": 1, "passed": 0, "cases": [{"name": "smoke", "status": "fail"}]})]
        with tempfile.TemporaryDirectory() as directory:
            output_path = os.path.join(directory, "outputs")
            summary_path = os.path.join(directory, "summary")
            with patch_candidate_env(GITHUB_REPOSITORY="goplus/spx", GITHUB_OUTPUT=output_path,
                                     GITHUB_STEP_SUMMARY=summary_path), \
                 mock.patch.object(run_spxbench_suite, "create_candidate_release"), \
                 mock.patch.object(run_spxbench_suite, "delete_candidate_release") as deleted, \
                 mock.patch.object(run_spxbench_suite, "request_spxbench") as removed, \
                 mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                                   return_value=SuiteJobEventsTest.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")), \
                 mock.patch.object(SpxbenchSuiteGate, "request", side_effect=responses):
                with self.assertRaisesRegex(SpxbenchSuiteError, "failed suite"):
                    run_spxbench_suite.main()
            with open(output_path) as output:
                self.assertEqual(output.read().splitlines(), ["candidate_install_started=true"])
            with open(summary_path) as summary:
                report = summary.read()
            self.assertIn("Regression: **failed**", report)
            self.assertIn("spxbench candidate: **retained**", report)
            self.assertIn("GitHub prerelease: **retained**", report)
        removed.assert_not_called()
        deleted.assert_not_called()

    def test_remove_failure_retains_prerelease(self):
        responses = [(200, {"suites": [{"id": 7, "name": "spx"}]}),
                     (200, {"versions": [{"kind": "spx", "version": "0.0.1-rc.1.1"}]}),
                     (200, {"jobId": 11}),
                     (200, {"jobId": 11, "running": False, "summary": "passed", "total": 1,
                            "completed": 1, "passed": 1, "cases": [{"name": "smoke", "status": "pass"}]})]
        with tempfile.TemporaryDirectory() as directory:
            summary_path = os.path.join(directory, "summary")
            with patch_candidate_env(GITHUB_REPOSITORY="goplus/spx", GITHUB_STEP_SUMMARY=summary_path), \
                 mock.patch.object(run_spxbench_suite, "create_candidate_release"), \
                 mock.patch.object(run_spxbench_suite, "delete_candidate_release") as deleted, \
                 mock.patch.object(run_spxbench_suite, "request_spxbench", return_value=(500, "unavailable")), \
                 mock.patch.object(run_spxbench_suite.urllib.request, "urlopen",
                                   return_value=SuiteJobEventsTest.stream(b"event: finished\ndata: {\"jobId\":11}\n\n")), \
                 mock.patch.object(SpxbenchSuiteGate, "request", side_effect=responses):
                run_spxbench_suite.main()
            with open(summary_path) as summary:
                report = summary.read()
            self.assertIn("Regression: **passed**", report)
            self.assertIn("spxbench candidate: **failed**", report)
            self.assertIn("GitHub prerelease: **retained**", report)
            self.assertIn("HTTP 500", report)
        deleted.assert_not_called()


if __name__ == "__main__":
    unittest.main()
