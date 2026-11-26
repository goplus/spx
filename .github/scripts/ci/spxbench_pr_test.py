"""Regression tests for consuming unprivileged PR artifacts in a trusted run."""

from copy import deepcopy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import spxbench_pr
from test_support import WORKFLOWS, action_steps, job_condition, job_needs, load_workflow


HEAD_SHA = "a" * 40
MERGE_SHA = "b" * 40
BASE_SHA = "c" * 40
REPOSITORY = "goplus/spx"


class PRCandidateTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        self.event_path = self.directory / "event.json"
        self.output_path = self.directory / "output"
        self.addCleanup(mock.patch.stopall)
        mock.patch.dict(os.environ, {
            "GITHUB_EVENT_PATH": str(self.event_path),
            "GITHUB_OUTPUT": str(self.output_path),
            "GITHUB_REPOSITORY": REPOSITORY,
            "GITHUB_RUN_ID": "200",
            "PR_HEAD_SHA": HEAD_SHA,
            "PREPARE_RESULT": "success",
            "SUITE_RESULT": "success",
        }, clear=True).start()
        self.api = mock.patch.object(spxbench_pr, "github_api", side_effect=self.github_api).start()
        self.reset()

    def reset(self):
        self.run = {
            "id": 100, "run_attempt": 1, "repository": {"full_name": REPOSITORY},
            "event": "pull_request", "path": ".github/workflows/ci.yml",
            "status": "completed", "conclusion": "success", "head_sha": HEAD_SHA,
            "head_branch": "feature", "head_repository": {"id": 7, "full_name": "author/spx"},
            "pull_requests": [], "display_title": "CI for PR #42 into dev",
        }
        self.event = {"repository": {"full_name": REPOSITORY}, "workflow_run": self.run}
        self.pr = {
            "number": 42, "state": "open",
            "base": {"ref": "dev", "repo": {"full_name": REPOSITORY}},
            "head": {"sha": HEAD_SHA, "ref": "feature", "repo": {"id": 7}},
        }
        self.artifact = {
            "id": 300, "name": f"spxbench-pr-42-{MERGE_SHA}-4.0.3", "expired": False,
        }
        self.current_run = deepcopy(self.run)
        self.runs = [self.current_run]
        self.other_pr = {**deepcopy(self.pr), "number": 43}
        self.artifacts = [self.artifact]
        self.merge = {"parents": [{"sha": BASE_SHA}, {"sha": HEAD_SHA}]}
        self.statuses = []
        self.api.reset_mock()
        self.output_path.write_text("")
        self.write_event()

    def write_event(self):
        self.event_path.write_text(json.dumps(self.event))

    def github_api(self, path, payload=None, paginate=False):
        if path == f"statuses/{HEAD_SHA}":
            self.assertFalse(paginate)
            self.statuses.append(payload)
            return {}
        self.assertIsNone(payload)
        responses = {
            "actions/runs/100": (False, self.current_run),
            f"actions/workflows/ci.yml/runs?event=pull_request&head_sha={HEAD_SHA}&per_page=100":
                (True, [{"workflow_runs": self.runs}]),
            "pulls/42": (False, self.pr),
            "pulls/43": (False, self.other_pr),
            "actions/runs/100/artifacts?per_page=100": (True, [{"artifacts": self.artifacts}]),
            f"git/commits/{MERGE_SHA}": (False, self.merge),
        }
        expected_paginate, result = responses[path]
        self.assertEqual(paginate, expected_paginate)
        return deepcopy(result)

    def outputs(self):
        return dict(line.split("=", 1) for line in self.output_path.read_text().splitlines())

    def test_fork_pr_without_event_pull_requests_uses_run_title(self):
        spxbench_pr.prepare()
        self.assertEqual(self.outputs(), {
            "head_sha": HEAD_SHA, "artifact_id": "300",
            "target_sha": MERGE_SHA, "version": "4.0.3",
        })
        self.api.assert_any_call("pulls/42")
        self.assertEqual(self.statuses, [{
            "state": "pending", "context": "spxbench", "description": "Validating the CI Web candidate",
            "target_url": "https://github.com/goplus/spx/actions/runs/200",
        }])

    def test_ci_start_and_rerun_mark_pending_without_consuming_artifacts(self):
        for attempt in (1, 2):
            with self.subTest(attempt=attempt):
                self.reset()
                self.run.update(status="in_progress", run_attempt=attempt, conclusion=None)
                self.current_run = deepcopy(self.run)
                self.write_event()
                spxbench_pr.prepare()
                self.assertEqual(self.outputs(), {})
                self.assertEqual([status["state"] for status in self.statuses], ["pending"])
                self.assertIn("Waiting for CI", self.statuses[0]["description"])
                self.assertNotIn(mock.call("actions/runs/100/artifacts?per_page=100", paginate=True),
                                 self.api.call_args_list)

    def test_late_or_superseded_start_does_not_reset_the_status(self):
        for obsolete in ("completed", "new_attempt", "new_run"):
            with self.subTest(obsolete=obsolete):
                self.reset()
                self.run["status"] = "in_progress"
                if obsolete != "completed":
                    self.current_run["status"] = "in_progress"
                    if obsolete == "new_attempt":
                        self.current_run["run_attempt"] = 2
                    else:
                        self.runs.append({**deepcopy(self.run), "id": 101})
                self.write_event()
                spxbench_pr.prepare()
                self.assertEqual(self.outputs(), {})
                self.assertEqual(self.statuses, [])

    def test_untrusted_or_incomplete_run_cannot_reach_github_api(self):
        for field, value in (
            ("repository", {"full_name": "author/spx"}), ("event", "push"),
            ("path", ".github/workflows/other.yml"), ("status", "queued"),
            ("head_sha", "not-a-sha"),
        ):
            with self.subTest(field=field):
                self.reset()
                self.run[field] = value
                self.write_event()
                with self.assertRaisesRegex(ValueError, "Expected a canonical PR CI run"):
                    spxbench_pr.prepare()
                self.api.assert_not_called()
        for location in ("environment", "event"):
            with self.subTest(repository_location=location):
                self.reset()
                if location == "environment":
                    with mock.patch.dict(os.environ, {"GITHUB_REPOSITORY": "author/spx"}):
                        with self.assertRaises(ValueError):
                            spxbench_pr.prepare()
                else:
                    self.event["repository"]["full_name"] = "author/spx"
                    self.write_event()
                    with self.assertRaises(ValueError):
                        spxbench_pr.prepare()
                self.api.assert_not_called()

    def test_pr_must_match_open_dev_branch_head_and_fork_repository(self):
        variants = [
            {"state": "closed"},
            {"base": {"ref": "main", "repo": {"full_name": REPOSITORY}}},
            {"base": {"ref": "dev", "repo": {"full_name": "author/spx"}}},
            {"head": {"sha": BASE_SHA, "ref": "feature", "repo": {"id": 7}}},
            {"head": {"sha": HEAD_SHA, "ref": "other", "repo": {"id": 7}}},
            {"head": {"sha": HEAD_SHA, "ref": "feature", "repo": {"id": 8}}},
            {"head": {"sha": HEAD_SHA, "ref": "feature", "repo": None}},
        ]
        for update in variants:
            with self.subTest(update=update):
                self.reset()
                self.pr.update(update)
                spxbench_pr.prepare()
                self.assertEqual(self.outputs(), {})
                self.assertEqual(self.statuses, [])

    def test_candidate_must_match_pr_identified_by_run_title(self):
        self.run["display_title"] = "CI for PR #43 into dev"
        self.write_event()
        with self.assertRaisesRegex(ValueError, "identify this PR"):
            spxbench_pr.prepare()
        self.api.assert_any_call("pulls/43")
        self.assertNotIn("artifact_id", self.outputs())

    def test_non_dev_or_malformed_title_is_ignored(self):
        for title in ("CI for PR #42 into main", "CI for PR #42 into dev\nextra", "CI", ""):
            with self.subTest(title=title):
                self.reset()
                self.run["display_title"] = title
                self.write_event()
                spxbench_pr.prepare()
                self.assertEqual(self.outputs(), {})
                self.assertEqual(self.statuses, [])
                self.assertNotIn(mock.call("pulls/42"), self.api.call_args_list)

    def test_failed_ci_never_runs_the_candidate(self):
        for conclusion in ("failure", "cancelled", "timed_out", "skipped", None):
            with self.subTest(conclusion=conclusion):
                self.reset()
                self.run["conclusion"] = conclusion
                self.write_event()
                with self.assertRaisesRegex(ValueError, "Required CI run ended"):
                    spxbench_pr.prepare()
                self.assertEqual(self.outputs(), {"head_sha": HEAD_SHA})
                self.assertEqual(self.statuses[-1]["state"], "pending")

    def test_candidate_must_be_unique_unexpired_and_for_this_pr(self):
        cases = (
            ([], "exactly one"),
            ([{"expired": True}], "exactly one"),
            ([{}, {"id": 301}], "exactly one"),
            ([{"name": f"spxbench-pr-43-{MERGE_SHA}-4.0.3"}], "identify this PR"),
            ([{"name": f"spxbench-pr-42-{MERGE_SHA}-4.0.3\nmalicious=true"}], "identify this PR"),
            ([{"name": f"spxbench-pr-42-{MERGE_SHA}-invalid"}], "identify this PR"),
        )
        for updates, message in cases:
            with self.subTest(updates=updates):
                self.reset()
                self.artifacts = [{**self.artifact, **update} for update in updates]
                with self.assertRaisesRegex(ValueError, message):
                    spxbench_pr.prepare()
                self.assertEqual(self.outputs(), {"head_sha": HEAD_SHA})

    def test_unrelated_coverage_artifact_does_not_conflict_with_candidate(self):
        self.artifacts.append({"id": 301, "name": "coverage-source", "expired": False})
        spxbench_pr.prepare()
        self.assertEqual(self.outputs()["artifact_id"], "300")

    def test_candidate_merge_must_have_pr_head_as_second_parent(self):
        for parents in ([], [{"sha": HEAD_SHA}], [{"sha": HEAD_SHA}, {"sha": BASE_SHA}],
                        [{"sha": BASE_SHA}, {"sha": HEAD_SHA}, {"sha": MERGE_SHA}]):
            with self.subTest(parents=parents):
                self.reset()
                self.merge["parents"] = parents
                with self.assertRaisesRegex(ValueError, "not a merge of this PR head"):
                    spxbench_pr.prepare()
                self.assertNotIn("artifact_id", self.outputs())

    def test_successful_ci_without_candidate_reports_failure(self):
        self.artifacts = []
        with self.assertRaisesRegex(ValueError, "exactly one"):
            spxbench_pr.prepare()
        self.assertEqual(self.outputs(), {"head_sha": HEAD_SHA})
        with mock.patch.dict(os.environ, {"PREPARE_RESULT": "failure", "SUITE_RESULT": "skipped"}):
            spxbench_pr.finish()
        self.assertEqual([status["state"] for status in self.statuses], ["pending", "failure"])

    def test_rerun_reuses_successful_artifact_from_previous_attempt(self):
        self.run["run_attempt"] = 2
        self.current_run["run_attempt"] = 2
        self.artifact["workflow_run"] = {"id": 100, "head_sha": HEAD_SHA}
        self.artifact["created_at"] = "2026-11-23T01:00:00Z"
        self.write_event()
        spxbench_pr.prepare()
        self.assertEqual(self.outputs()["artifact_id"], "300")

    def test_superseded_attempt_or_run_is_ignored(self):
        for superseded in ("attempt", "running", "new_run"):
            with self.subTest(superseded=superseded):
                self.reset()
                if superseded == "attempt":
                    self.current_run["run_attempt"] = 2
                elif superseded == "running":
                    self.current_run["status"] = "in_progress"
                else:
                    self.runs.append({**deepcopy(self.run), "id": 101})
                spxbench_pr.prepare()
                self.assertEqual(self.outputs(), {})
                self.assertEqual(self.statuses, [])

    def test_other_fork_branch_or_pr_run_does_not_supersede_candidate(self):
        for update in (
            {"head_repository": {"id": 8}}, {"head_branch": "other"},
            {"display_title": "CI for PR #42 into main"}, {"display_title": "CI for PR #43 into dev"},
        ):
            with self.subTest(update=update):
                self.reset()
                self.runs.append({**deepcopy(self.run), "id": 101, **update})
                spxbench_pr.prepare()
                self.assertEqual(self.outputs()["artifact_id"], "300")

    def test_finish_requires_successful_candidate_validation_and_suite(self):
        for prepare, suite, state, description in (
            ("success", "success", "success", "regression suite passed"),
            ("success", "failure", "failure", "failed or was interrupted"),
            ("success", "cancelled", "failure", "failed or was interrupted"),
            ("success", "skipped", "failure", "failed or was interrupted"),
            ("failure", "skipped", "failure", "candidate validation failed"),
        ):
            with self.subTest(prepare=prepare, suite=suite):
                self.reset()
                with mock.patch.dict(os.environ, {"PREPARE_RESULT": prepare, "SUITE_RESULT": suite}):
                    spxbench_pr.finish()
                self.assertEqual(len(self.statuses), 1)
                self.assertEqual(self.statuses[0]["state"], state)
                self.assertIn(description, self.statuses[0]["description"])

    def test_finish_does_not_overwrite_obsolete_or_closed_pr_result(self):
        for obsolete in ("new_run", "closed", "new_head"):
            with self.subTest(obsolete=obsolete):
                self.reset()
                if obsolete == "new_run":
                    self.runs.append({**deepcopy(self.run), "id": 101})
                elif obsolete == "closed":
                    self.pr["state"] = "closed"
                else:
                    self.pr["head"]["sha"] = BASE_SHA
                spxbench_pr.finish()
                self.assertEqual(self.statuses, [])

    def test_finish_rejects_a_different_status_target(self):
        with mock.patch.dict(os.environ, {"PR_HEAD_SHA": BASE_SHA}):
            with self.assertRaisesRegex(ValueError, "Status target does not match"):
                spxbench_pr.finish()
        self.assertEqual(self.statuses, [])


class PRWorkflowTest(unittest.TestCase):
    def test_trusted_consumer_downloads_validated_artifact_and_reports_even_on_failure(self):
        ci = load_workflow(WORKFLOWS / "ci.yml")
        self.assertIn("format('CI for PR #{0} into {1}', github.event.pull_request.number, github.base_ref)",
                      ci["run-name"])
        workflow = load_workflow(WORKFLOWS / "spxbench_pr.yml")
        self.assertEqual(workflow["on"], {
            "workflow_run": {"workflows": ["CI"], "types": ["in_progress", "completed"]},
        })
        self.assertIn("${{ github.event.action }}", workflow["concurrency"]["group"])
        jobs = workflow["jobs"]
        self.assertIn("github.event.workflow_run.event=='pull_request'", job_condition(jobs["prepare"]))
        self.assertCountEqual(job_needs(jobs["report"]), ("prepare", "run-spxbench-suite"))
        self.assertIn("always()", job_condition(jobs["report"]))
        for name in ("prepare", "report"):
            job = jobs[name]
            self.assertEqual(job.get("permissions", workflow["permissions"]), {
                "contents": "read", "actions": "read", "pull-requests": "read", "statuses": "write",
            })
            checkout, = action_steps(job, "actions/checkout")
            self.assertEqual(checkout["with"], {"ref": "${{ github.sha }}", "persist-credentials": "false"})
        suite = jobs["run-spxbench-suite"]
        self.assertEqual(job_condition(suite), "needs.prepare.outputs.artifact_id!=''")
        self.assertEqual(suite["uses"], "./.github/workflows/run_spxbench_suite.yml")
        self.assertEqual(suite["permissions"], {"contents": "read", "actions": "read"})
        self.assertEqual(suite["with"]["artifact_id"], "${{ needs.prepare.outputs.artifact_id }}")
        self.assertEqual(suite["with"]["artifact_run_id"], "${{ github.event.workflow_run.id }}")
        self.assertEqual(suite["with"]["target_sha"], "${{ needs.prepare.outputs.target_sha }}")
        self.assertEqual(suite["secrets"], {"SPXBENCH_API_TOKEN": "${{ secrets.SPXBENCH_API_TOKEN }}"})


if __name__ == "__main__":
    unittest.main()
