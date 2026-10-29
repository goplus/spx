#!/usr/bin/env python3

import json
from pathlib import Path
import sys
import tempfile
import unittest

from test_support import (SCRIPTS, WORKFLOWS, action_steps, job_condition, job_needs,
                        load_workflow, named_step, run_command, run_shell,
                        step_script)

CI_WORKFLOW_PATH = WORKFLOWS / "ci.yml"
SOURCE_CHECKS_WORKFLOW_PATH = WORKFLOWS / "check_source.yml"
GO_BUILD_WORKFLOW_PATH = WORKFLOWS / "build_go.yml"
WEB_BUILD_WORKFLOW_PATH = WORKFLOWS / "build_web.yml"
NPM_WORKFLOW_PATH = WORKFLOWS / "publish_npm.yml"
MODULE_WORKFLOW_PATH = WORKFLOWS / "test_godot_module.yml"


class SourceChecksWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.job = load_workflow(SOURCE_CHECKS_WORKFLOW_PATH)["jobs"]["check-source"]

    def test_suites_forward_coverage_outputs_and_keep_native_platforms(self):
        suites = {
            "web": "web-tests", "web-process": "web-process-tests", "web-interpreter": "web-interpreter-tests",
            "tools": "tool-tests", "codegen": "codegen-tests", "native": "go-tests",
        }
        steps = self.job["steps"]
        artifact = action_steps(self.job, "actions/upload-artifact")[0]
        for suite, step_id in suites.items():
            step = next(step for step in steps if step.get("id") == step_id)
            self.assertEqual(step["run"], f"bash .github/scripts/ci/test_go_coverage.sh {suite}")
            self.assertNotIn("working-directory", step)
            self.assertFalse({"GOOS", "GOARCH"} & step.get("env", {}).keys())
            self.assertIn(f"${{{{ steps.{step_id}.outputs.coverage }}}}", artifact["with"]["path"])
            self.assertLess(steps.index(step), steps.index(artifact))
        self.assertEqual(artifact["if"], "inputs.collect_coverage")
        for key, value in {"name": "coverage-source", "if-no-files-found": "error", "overwrite": "true"}.items():
            self.assertEqual(artifact["with"][key], value)
        abi = named_step(self.job, "Test Web ABI memory ownership")
        self.assertEqual(abi["env"]["GDSPX_SANITIZERS"], "address,undefined")
        self.assertIn("bash godot_modules/spx/web/tests/run_string_abi_test.sh", abi["run"])


class CoverageWorkflowTest(unittest.TestCase):
    def test_coverage_is_uploaded_only_after_the_selected_ci_path_passes(self):
        jobs = load_workflow(CI_WORKFLOW_PATH)["jobs"]
        coverage = jobs["ci-gate"]
        self.assertCountEqual(job_needs(coverage), (
            "check-source", "build-go", "resolve-runtime", "test-godot-module", "build-web",
        ))
        self.assertEqual(job_condition(coverage), "!cancelled()")
        self.assertEqual(jobs["check-source"]["with"]["collect_coverage"], "true")
        go, cpp, finalize = action_steps(coverage, "codecov/codecov-action")
        for key, value in {"files": "${{ steps.profiles.outputs.coverage }}", "slug": "goplus/spx",
                           "flags": "go", "disable_search": "true"}.items():
            self.assertEqual(go["with"][key], value)
        self.assertEqual(cpp["with"], {
            "slug": "goplus/spx", "files": "coverage/cpp/coverage.xml", "flags": "cpp",
            "disable_search": "true", "fail_ci_if_error": "true",
        })
        self.assertEqual(finalize["with"], {
            "slug": "goplus/spx", "run_command": "send-notifications", "fail_ci_if_error": "true",
        })
        for path in CI_WORKFLOW_PATH.parent.glob("*.yml"):
            count = sum(len(action_steps(job, "codecov/codecov-action"))
                        for job in load_workflow(path)["jobs"].values() if "steps" in job)
            self.assertEqual(count, 3 if path == CI_WORKFLOW_PATH else 0, path.name)
        go_download, cpp_download = action_steps(coverage, "actions/download-artifact")
        self.assertEqual(go_download["with"], {"pattern": "coverage-{source,macos,windows}", "path": "coverage"})
        self.assertEqual(cpp_download["with"], {"name": "coverage-spx-cpp", "path": "coverage/cpp"})
        collector = named_step(coverage, "Collect Go coverage profiles")
        self.assertEqual(collector["run"], 'python3 .github/scripts/ci/collect_coverage.py coverage --github-output "$GITHUB_OUTPUT"')
        guard = named_step(coverage, "Verify required CI path passed")
        ordered = (guard, go_download, cpp_download, collector, go, cpp, finalize)
        indices = [coverage["steps"].index(step) for step in ordered]
        self.assertEqual(indices, sorted(indices))
        for step in ordered:
            self.assertNotIn("continue-on-error", step)
            condition = "needs.test-godot-module.result == 'success'" if step in (cpp, cpp_download) else "success()"
            self.assertEqual(step.get("if", "success()"), condition)
        for step in (go, cpp, finalize):
            self.assertEqual(step["with"]["fail_ci_if_error"], "true")
        static = load_workflow(SOURCE_CHECKS_WORKFLOW_PATH)
        self.assertEqual(static["on"]["workflow_call"]["inputs"]["collect_coverage"]["default"], "false")

    def test_module_workflow_keeps_its_gate_without_publishing_partial_coverage(self):
        workflow = load_workflow(MODULE_WORKFLOW_PATH)
        self.assertEqual(workflow["permissions"], {"contents": "read"})
        gate = workflow["jobs"]["integration-gate"]
        self.assertEqual(job_needs(gate), ("resolve-inputs", "test-module"))
        self.assertEqual(job_condition(gate), "!cancelled()")
        self.assertEqual(gate["steps"][0]["name"], "Verify integration checks passed")
        self.assertNotIn("continue-on-error", gate)
        for step in gate["steps"]:
            self.assertNotIn("continue-on-error", step)
            self.assertEqual(step.get("if", "success()"), "success()")

    def test_integration_gate_rejects_unsuccessful_builds(self):
        gate = load_workflow(MODULE_WORKFLOW_PATH)["jobs"]["integration-gate"]
        step = named_step(gate, "Verify integration checks passed")
        self.assertEqual(step["shell"], "python")
        self.assertEqual(step["env"], {"NEEDS_JSON": "${{ toJSON(needs) }}"})

        def run_gate(needs):
            return run_command([sys.executable, "-c", step["run"]], env={"NEEDS_JSON": json.dumps(needs)})

        success = {job: {"result": "success"} for job in ("resolve-inputs", "test-module")}
        result = run_gate(success)
        self.assertEqual(result.returncode, 0, result.stderr)
        for job in success:
            for status in ("failure", "cancelled", "skipped", "", None):
                with self.subTest(job=job, status=status):
                    result = run_gate({**success, job: {"result": status}})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(f"{job}: {status}", result.stderr)
            for needs in ({**success, job: {}}, {key: value for key, value in success.items() if key != job}):
                with self.subTest(job=job, missing_result=needs):
                    result = run_gate(needs)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(f"{job}: missing", result.stderr)

    def test_only_cpp_coverage_is_carried_forward_when_its_tests_are_skipped(self):
        config = load_workflow(WORKFLOWS.parent / "codecov.yml")
        self.assertEqual(config["flags"], {
            "go": {"carryforward": "false"},
            "cpp": {"paths": ["godot_modules/spx/"], "carryforward": "true"},
        })
        self.assertEqual(config["comment"]["show_carryforward_flags"], "true")
        self.assertEqual(config["codecov"]["notify"], {"manual_trigger": "true"})
        # The pre-migration Go baseline has no flags. Path filtering preserves
        # that comparison without excluding C++ from the aggregate report.
        for check in ("project", "patch"):
            self.assertEqual(config["coverage"]["status"][check]["default"], {
                "target": "auto", "paths": [r".*\.go$"],
            })

    def test_collection_requires_every_source_and_keeps_nested_profiles(self):
        script = SCRIPTS / "ci/collect_coverage.py"
        sources = {
            "source": ["coverage.out", "cmd/ispx/coverage.out", "internal/cmd/codegen/coverage.out",
                       "coverage-web.out", "cmd/ispx/coverage-web.out"],
            "macos": ["coverage-buildctl-macos.out"],
            "windows": ["coverage-buildctl-windows.out", "coverage-engine-windows.out"],
        }
        for missing in (None, *sources):
            for empty_directory in (False, True) if missing else (False,):
                with self.subTest(missing=missing, empty_directory=empty_directory), tempfile.TemporaryDirectory(prefix="spx coverage ") as directory:
                    root = Path(directory)
                    output = root / "output"
                    # Unknown artifacts and local reports must not be uploaded.
                    for name in ("coverage.out", "coverage/coverage-unrelated/coverage.out",
                                 "coverage/coverage-spx-cpp/coverage.xml"):
                        path = root / name
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_text("unrelated\n")
                    profiles = []
                    for source, names in sources.items():
                        source_dir = root / "coverage" / f"coverage-{source}"
                        if source == missing:
                            if empty_directory:
                                source_dir.mkdir(parents=True)
                            continue
                        for name in names:
                            path = source_dir / name
                            path.parent.mkdir(parents=True, exist_ok=True)
                            path.write_text("mode: atomic\n")
                            profiles.append(path.relative_to(root).as_posix())
                    result = run_command([sys.executable, str(script), "coverage", "--github-output", str(output)], cwd=root)
                    if missing:
                        self.assertNotEqual(result.returncode, 0)
                        self.assertIn(f"No coverage profiles found for {missing}", result.stderr)
                        self.assertFalse(output.exists())
                    else:
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertEqual(output.read_text(), f"coverage={','.join(sorted(profiles))}\n")


class GoBuildWorkflowTest(unittest.TestCase):
    def test_native_coverage_uses_build_runners_and_unique_uploads(self):
        job = load_workflow(GO_BUILD_WORKFLOW_PATH)["jobs"]["build-go"]
        targets = job["strategy"]["matrix"]["include"]
        self.assertEqual({target["coverage-suite"] for target in targets}, {"", "macos", "windows"})
        self.assertEqual(job["runs-on"], "${{ matrix.os }}")
        test = named_step(job, "Test native platform behavior")
        self.assertEqual(test["id"], "native-tests")
        self.assertEqual(test["env"]["COVERAGE_SUITE"], "${{ matrix.coverage-suite }}")
        self.assertEqual(test["run"], 'bash .github/scripts/ci/test_go_coverage.sh "$COVERAGE_SUITE"')
        upload = named_step(job, "Save native coverage for combined upload")
        for step in (test, upload):
            self.assertEqual(step["if"], "matrix.coverage-suite != ''")
        self.assertEqual(upload["with"]["name"], "coverage-${{ matrix.coverage-suite }}")
        self.assertEqual(upload["with"]["path"], "${{ steps.native-tests.outputs.coverage }}")
        self.assertEqual(upload["with"]["overwrite"], "true")
        self.assertEqual(upload["with"]["if-no-files-found"], "error")
        self.assertLess(job["steps"].index(test), job["steps"].index(upload))
        self.assertNotIn("continue-on-error", job)
        for step in job["steps"]:
            self.assertFalse({"GOOS", "GOARCH"} & step.get("env", {}).keys())
        web = named_step(job, "Build cmd/ispx")
        self.assertEqual(web["if"], "runner.os == 'Linux'")
        self.assertEqual(web["run"], "./build.sh --base")
        self.assertEqual(web["working-directory"], "cmd/ispx")
        self.assertEqual(named_step(job, "Build cmd/ispxnative")["run"], "./build.sh")

    def test_build_discovery_preserves_packages_and_stops_on_failure(self):
        job = load_workflow(GO_BUILD_WORKFLOW_PATH)["jobs"]["build-go"]
        script = step_script(job, "Build Go packages")
        trace = '''go() {
  if [ "$1" = list ]; then
    printf '%s\\n' github.com/goplus/spx/v4 github.com/goplus/spx/v4/cmd/ispxnative github.com/goplus/spx/v4/fs
    return "${LIST_EXIT:-0}"
  fi
  printf '%s\\n' "$*"
}
''' + script
        for code in (0, 7):
            with self.subTest(code=code):
                result = run_shell(trace, env={"LIST_EXIT": str(code)})
                self.assertEqual(result.returncode, code, result.stderr)
                self.assertEqual(result.stdout, "build -v github.com/goplus/spx/v4 github.com/goplus/spx/v4/fs\n" if code == 0 else "")


class ProductBuildWorkflowTest(unittest.TestCase):
    def test_required_checks_build_and_test_only_base_modules(self):
        for path, name in ((SOURCE_CHECKS_WORKFLOW_PATH, "check-source"), (GO_BUILD_WORKFLOW_PATH, "build-go")):
            job = load_workflow(path)["jobs"][name]
            self.assertEqual(job["env"]["GOWORK"], "off")
            self.assertNotIn("cmd/ispx/ai", json.dumps(job))
        static = load_workflow(SOURCE_CHECKS_WORKFLOW_PATH)["jobs"]["check-source"]
        check = named_step(static, "Ensure generated files are up to date")
        self.assertEqual(check["env"]["CHECK_EXPORT_TYPES"], "${{ inputs.check_export_types }}")
        self.assertEqual(check["run"], 'bash .github/scripts/ci/check_generated.sh --check-export-types "$CHECK_EXPORT_TYPES"')

    def test_web_builds_share_one_action(self):
        for path, job_name in ((WEB_BUILD_WORKFLOW_PATH, "build-web"),
                               (WORKFLOWS / "package_web.yml", "package-web"),
                               (NPM_WORKFLOW_PATH, "publish-npm")):
            with self.subTest(workflow=path.name):
                job = load_workflow(path)["jobs"][job_name]
                self.assertNotIn("SPX_ISPX_PROFILE", job.get("env", {}))
                builds = action_steps(job, "./.github/actions/build-web-bundle")
                self.assertEqual(len(builds), 1)
                if path == NPM_WORKFLOW_PATH:
                    self.assertEqual(builds[0]["if"], "inputs.release_artifact == ''")
                for step in job["steps"]:
                    self.assertNotIn("SPX_ISPX_PROFILE", step.get("env", {}))
                    self.assertNotIn("runtime export-web", step.get("run", ""))


class CIWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.jobs = load_workflow(CI_WORKFLOW_PATH)["jobs"]

    def test_ci_selects_current_build_for_an_incompatible_runtime(self):
        resolution = step_script(self.jobs["resolve-runtime"], "Resolve runtime release")
        self.assertIn("go run ./.github/cmd/runtime-resolution", resolution)
        self.assertIn('--github-output "$GITHUB_OUTPUT"', resolution)

        self.assertEqual(
            job_condition(self.jobs["test-godot-module"]),
            "needs.resolve-runtime.outputs.runtime_state=='missing'||"
            "needs.resolve-runtime.outputs.runtime_state=='incompatible'",
        )
        self.assertEqual(
            job_condition(self.jobs["build-web"]),
            "needs.resolve-runtime.outputs.runtime_state=='ready'",
        )

        job = self.jobs["ci-gate"]
        checkouts = action_steps(job, "actions/checkout")
        self.assertEqual(len(checkouts), 1)
        self.assertEqual(checkouts[0]["with"]["persist-credentials"], "false")
        step = named_step(job, "Verify required CI path passed")
        self.assertLess(job["steps"].index(checkouts[0]), job["steps"].index(step))
        self.assertEqual(step["run"], "python3 .github/scripts/ci/check_results.py")
        result_jobs = ("check-source", "build-go", "resolve-runtime", "test-godot-module", "build-web")
        self.assertEqual(job_condition(job), "!cancelled()")
        self.assertCountEqual(job_needs(job), result_jobs)
        self.assertEqual(step["env"], {"NEEDS_JSON": "${{ toJSON(needs) }}"})

        def run_gate(needs):
            return run_command([sys.executable, str(SCRIPTS / "ci/check_results.py")],
                               env={"NEEDS_JSON": json.dumps(needs)})

        for state in ("ready", "missing", "incompatible"):
            expected = {job: {"result": "success"} for job in result_jobs}
            expected["test-godot-module"]["result"] = "skipped" if state == "ready" else "success"
            expected["build-web"]["result"] = "success" if state == "ready" else "skipped"
            runtime = {"runtime_state": state, "runtime_reason": "test runtime resolution"}
            expected["resolve-runtime"]["outputs"] = runtime
            with self.subTest(state=state):
                completed = run_gate(expected)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                message = ("Published runtime verified; Web normal bundle built" if state == "ready" else
                           f"Published runtime is {state}; current SPX module integration passed")
                self.assertEqual(completed.stdout, f"[ok] {message}\n[info] Runtime resolution: test runtime resolution\n")
            for job in result_jobs:
                for result in ("success", "failure", "cancelled", "skipped", "", None):
                    if result == expected[job]["result"]:
                        continue
                    with self.subTest(state=state, job=job, result=result):
                        completed = run_gate({**expected, job: {**expected[job], "result": result}})
                        self.assertNotEqual(completed.returncode, 0)
                        self.assertIn("want " + expected[job]["result"], completed.stderr)
                with self.subTest(state=state, missing_job=job):
                    completed = run_gate({key: value for key, value in expected.items() if key != job})
                    self.assertEqual(completed.returncode, 1)
                    self.assertIn(f"{job} result is missing", completed.stderr)
                    self.assertEqual(completed.stdout, "")
            for variable in runtime:
                with self.subTest(state=state, missing_output=variable):
                    outputs = {key: value for key, value in runtime.items() if key != variable}
                    completed = run_gate({**expected, "resolve-runtime": {"result": "success", "outputs": outputs}})
                    self.assertEqual(completed.returncode, 1)
                    self.assertIn(f"Missing required input: {variable}", completed.stderr)
                    self.assertEqual(completed.stdout, "")
            # A provided empty reason remains valid.
            runtime["runtime_reason"] = ""
            completed = run_gate(expected)
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertTrue(completed.stdout.endswith("[info] Runtime resolution: \n"))
            for invalid_state in ("unknown", ""):
                with self.subTest(state=invalid_state):
                    runtime["runtime_state"] = invalid_state
                    completed = run_gate(expected)
                    self.assertNotEqual(completed.returncode, 0)
                    self.assertIn("Unknown runtime state", completed.stderr)


if __name__ == "__main__":
    unittest.main()
