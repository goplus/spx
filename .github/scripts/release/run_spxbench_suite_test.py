"""Unit tests for the spxbench release suite gate."""

import os
import unittest
from unittest import mock

import run_spxbench_suite
from run_spxbench_suite import (SpxbenchSuiteError, SpxbenchSuiteGate, failing_cases,
                                find_suite_id, is_run_slot_busy, job_failure_reasons, require_env)


CANDIDATE_ENV = {"SPXBENCH_URL": "http://127.0.0.1", "SPXBENCH_API_TOKEN": "t",
                 "WEB_ARCHIVE_PATH": "dist/spx_web.zip", "CANDIDATE_VERSION": "0.0.1-rc.1.1", "GITHUB_SHA": "abc123"}


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
        with mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True), \
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
                 mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True), \
                 mock.patch.object(SpxbenchSuiteGate, "request", return_value=(status, body)) as requested, \
                 mock.patch.object(run_spxbench_suite.time, "sleep") as sleep:
                with self.assertRaises(SpxbenchSuiteError) as caught:
                    SpxbenchSuiteGate().start_suite_run(7)
                self.assertEqual(str(caught.exception), message)
                requested.assert_called_once_with("/__e2e/api/run?suite=7&spxVersion=0.0.1-rc.1.1", method="POST")
                sleep.assert_not_called()


class CandidateInstallTest(unittest.TestCase):
    def test_install_response_must_include_candidate(self):
        with mock.patch.dict(os.environ, {**CANDIDATE_ENV, "GITHUB_REPOSITORY": "goplus/spx"}, clear=True), \
             mock.patch.object(SpxbenchSuiteGate, "request", return_value=(200, {"versions": []})):
            gate = SpxbenchSuiteGate()
            with self.assertRaisesRegex(SpxbenchSuiteError, "did not include candidate"):
                gate.install_candidate()


class SuiteJobPollingTest(unittest.TestCase):
    def test_only_the_requested_finished_job_is_returned(self):
        states = [{"jobId": 12, "running": False},
                  {"jobId": 11, "running": True},
                  {"jobId": 11, "running": False}]
        with mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True), \
             mock.patch.object(run_spxbench_suite.time, "monotonic", return_value=100), \
             mock.patch.object(run_spxbench_suite.time, "sleep") as sleep, \
             mock.patch.object(SpxbenchSuiteGate, "request",
                               side_effect=[(200, state) for state in states]) as requested:
            self.assertIs(SpxbenchSuiteGate().wait_for_suite_job(11), states[-1])
        self.assertEqual(requested.call_args_list, [mock.call("/__e2e/api/state?job=11")] * 3)
        self.assertEqual(sleep.call_args_list, [mock.call(30)] * 2)


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
        with mock.patch.dict(os.environ, {**CANDIDATE_ENV, "SPXBENCH_TIMEOUT_MINUTES": "zero"}, clear=True):
            with self.assertRaisesRegex(SpxbenchSuiteError, "positive integer"):
                SpxbenchSuiteGate()

    def test_busy_retry_stops_at_deadline(self):
        with mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True), \
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
        with mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True):
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
        with mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True), \
             mock.patch.object(run_spxbench_suite, "create_candidate_release",
                               side_effect=SpxbenchSuiteError("boom")), \
             mock.patch.object(run_spxbench_suite, "delete_candidate_release") as deleted:
            with self.assertRaises(SpxbenchSuiteError):
                run_spxbench_suite.main()
            deleted.assert_called_once_with("v0.0.1-rc.1.1")

    def test_cleanup_failure_does_not_hide_creation_failure(self):
        with mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True), \
             mock.patch.object(run_spxbench_suite, "create_candidate_release",
                               side_effect=SpxbenchSuiteError("release creation failed")), \
             mock.patch.object(run_spxbench_suite, "delete_candidate_release", return_value="release not found"):
            with self.assertRaisesRegex(SpxbenchSuiteError, "release creation failed"):
                run_spxbench_suite.main()

    def test_successful_validation_fails_if_cleanup_fails(self):
        with mock.patch.dict(os.environ, CANDIDATE_ENV, clear=True), \
             mock.patch.object(run_spxbench_suite, "create_candidate_release"), \
             mock.patch.object(run_spxbench_suite, "delete_candidate_release", return_value="permission denied"), \
             mock.patch.object(SpxbenchSuiteGate, "resolve_suite", return_value=7), \
             mock.patch.object(SpxbenchSuiteGate, "install_candidate"), \
             mock.patch.object(SpxbenchSuiteGate, "start_suite_run", return_value=1), \
             mock.patch.object(SpxbenchSuiteGate, "wait_for_suite_job", return_value={"summary": "passed", "total": 1,
                                                                                 "completed": 1, "passed": 1, "cases": []}):
            with self.assertRaisesRegex(SpxbenchSuiteError, "release cleanup failed"):
                run_spxbench_suite.main()

    def test_candidate_is_pinned_across_release_install_and_run(self):
        responses = [(200, {"suites": [{"id": 7, "name": "spx"}]}),
                     (200, {"versions": [{"kind": "spx", "version": "0.0.1-rc.1.1", "digest": "abc"}]}),
                     (200, {"jobId": 11}),
                     (200, {"jobId": 11, "running": False, "summary": "passed", "total": 1,
                            "completed": 1, "passed": 1, "cases": [{"name": "smoke", "status": "pass"}]})]
        with mock.patch.dict(os.environ, {**CANDIDATE_ENV, "GITHUB_REPOSITORY": "goplus/spx",
                                       "GITHUB_SHA": "abc123"}, clear=True), \
             mock.patch.object(run_spxbench_suite, "create_candidate_release") as created, \
             mock.patch.object(run_spxbench_suite, "delete_candidate_release", return_value=None) as deleted, \
             mock.patch.object(SpxbenchSuiteGate, "request", side_effect=responses) as requested:
            run_spxbench_suite.main()
        created.assert_called_once_with("v0.0.1-rc.1.1", "dist/spx_web.zip", "abc123")
        deleted.assert_called_once_with("v0.0.1-rc.1.1")
        self.assertEqual(requested.call_args_list[1], mock.call("/api/runtime-versions", method="POST",
            payload={"action": "install", "kind": "spx", "version": "0.0.1-rc.1.1",
                     "sourceUrl": "https://github.com/goplus/spx/releases/download/v0.0.1-rc.1.1/spx_web.zip"}))
        self.assertEqual(requested.call_args_list[2],
                         mock.call("/__e2e/api/run?suite=7&spxVersion=0.0.1-rc.1.1", method="POST"))


if __name__ == "__main__":
    unittest.main()
