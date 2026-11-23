#!/usr/bin/env python3

import itertools
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import unittest

from test_support import (WORKFLOWS, action_steps, job_condition, job_needs,
                          load_workflow, named_step, run_command, run_shell, step_script,
                          write_executable)


class ReleaseWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = load_workflow(WORKFLOWS / "release.yml")
        cls.jobs = cls.workflow["jobs"]
        cls.build_workflow = load_workflow(WORKFLOWS / "build_release.yml")
        cls.build_jobs = cls.build_workflow["jobs"]

    def test_release_identity_permissions_and_input_contract(self):
        inputs = self.workflow["on"]["workflow_dispatch"]["inputs"]
        self.assertEqual(inputs["release_tag"]["required"], "false")
        self.assertEqual(inputs["release_tag"]["default"], "")
        self.assertEqual(inputs["operation"]["options"],
                         ["dry-run", "publish-runtime", "publish-release", "publish-dev-npm"])
        self.assertEqual(self.workflow["concurrency"], {"group": "spx-release", "cancel-in-progress": "false"})
        self.assertEqual(self.workflow["permissions"], {"contents": "read"})
        for name in ("publish-runtime", "prepare-spx-release", "publish-spx-release", "run-spxbench-suite"):
            self.assertEqual(self.jobs[name]["permissions"], {"contents": "write"})
        # npm trusted publishing keeps release.yml as its direct caller.
        for name in ("publish-npm", "publish-dev-npm"):
            self.assertEqual(self.jobs[name]["permissions"], {"contents": "read", "id-token": "write"})
            self.assertEqual(self.jobs[name]["uses"], "./.github/workflows/publish_npm.yml")
            self.assertEqual(self.jobs[name]["with"]["target"], "${{ github.sha }}")

    def test_build_boundary_exposes_only_consumed_verified_outputs(self):
        call = self.jobs["build-release"]
        self.assertEqual(call["uses"], "./.github/workflows/build_release.yml")
        self.assertEqual(call["with"], {name: f"${{{{ inputs.{name} }}}}"
                                       for name in ("operation", "release_tag", "platforms", "reuse_engine_version")})
        self.assertEqual(set(self.build_workflow["on"]), {"workflow_call"})
        self.assertEqual(self.build_workflow["permissions"], {"contents": "read"})
        self.assertNotIn("concurrency", self.build_workflow)
        self.assertTrue(all("permissions" not in job for job in self.build_jobs.values()))
        interface = self.build_workflow["on"]["workflow_call"]
        self.assertEqual(set(interface["inputs"]), set(call["with"]))
        for name, spec in interface["inputs"].items():
            self.assertEqual(spec["type"], "string")
            self.assertEqual(spec["required"], "false" if name == "reuse_engine_version" else "true")
        self.assertEqual(interface["inputs"]["reuse_engine_version"]["default"], "")
        outputs = {"release_tag", "version", "runtime_version", "runtime_tag",
                   "runtime_manifest", "runtime_state", "release_repository"}
        self.assertEqual(set(interface["outputs"]), outputs)
        self.assertEqual(set(re.findall(r"needs\.build-release\.outputs\.([a-z_]+)",
                                        (WORKFLOWS / "release.yml").read_text())), outputs)
        for name in outputs:
            self.assertEqual(interface["outputs"][name]["value"], f"${{{{ jobs.plan-release.outputs.{name} }}}}")

    def test_export_types_snapshot_is_checked_at_release(self):
        source = load_workflow(WORKFLOWS / "check_source.yml")
        options = source["on"]["workflow_call"]["inputs"]["check_export_types"]
        self.assertEqual(options["type"], "boolean")
        self.assertEqual(options["default"], "false")
        self.assertEqual(self.build_jobs["check-source"]["with"]["check_export_types"], "true")
        ci = load_workflow(WORKFLOWS / "ci.yml")
        self.assertNotIn("check_export_types", ci["jobs"]["check-source"].get("with", {}))
        check = named_step(source["jobs"]["check-source"], "Ensure generated files are up to date")
        self.assertEqual(check["env"]["CHECK_EXPORT_TYPES"], "${{ inputs.check_export_types }}")

    def test_scripts_are_wired_after_their_source_checkout(self):
        commands = {
            "plan-release": {
                "Validate release inputs": "release/plan.py inputs",
                "Enforce runtime reuse policy": "release/plan.py runtime-policy",
                "Summarize locked Godot source ancestry": "release/plan.py ancestry-summary",
                "Validate immutable publish targets": "release/publish.py validate-targets",
            },
            "validate-dev-npm": {
                "Require the canonical dev branch": "release/plan.py dev-target",
                "Require reusable development runtime": "release/plan.py dev-runtime-policy",
            },
            "prepare-spx-release": {"Prepare SPX release draft": "release/publish.py spx"},
            "run-spxbench-suite": {"Run spxbench regression suite": "release/run_spxbench_suite.py"},
        }
        for job_name, steps in commands.items():
            job = {**self.build_jobs, **self.jobs}[job_name]
            checkout, = action_steps(job, "actions/checkout")
            for name, command in steps.items():
                with self.subTest(job=job_name, step=name):
                    step = named_step(job, name)
                    self.assertEqual(step["run"], f"python3 .github/scripts/{command}")
                    self.assertLess(job["steps"].index(checkout), job["steps"].index(step))
        checkout, = action_steps(self.jobs["prepare-spx-release"], "actions/checkout")
        self.assertEqual(checkout["with"], {"ref": "${{ github.sha }}", "persist-credentials": "false"})
        planning = self.build_jobs["plan-release"]
        plan = named_step(planning, "Validate release inputs")
        self.assertEqual(plan["env"]["PLATFORMS_INPUT"], "${{ inputs.platforms }}")
        self.assertLess(planning["steps"].index(plan), planning["steps"].index(named_step(planning, "Resolve locked runtime release")))
        self.assertEqual(planning["outputs"]["package_platforms"],
                         "${{ steps." + plan["id"] + ".outputs.package_platforms }}")

    def test_ready_runtime_reuse_does_not_require_checkout_or_mutations(self):
        job = self.jobs["publish-runtime"]
        for name in ("Check out release source for the publish guard", "Download assembled runtime release",
                     "Revalidate locked Godot ancestry before publishing"):
            self.assertEqual(job_condition(named_step(job, name)), "needs.build-release.outputs.runtime_state=='missing'")
        toolchain, = action_steps(job, "./.github/actions/setup-toolchain")
        self.assertEqual(job_condition(toolchain), "needs.build-release.outputs.runtime_state=='missing'")
        self.assertLess(job["steps"].index(toolchain),
                        job["steps"].index(named_step(job, "Publish immutable runtime release")))
        with tempfile.TemporaryDirectory() as directory:
            result = run_shell(step_script(job, "Publish immutable runtime release"), cwd=directory,
                               env={"RUNTIME_STATE": "ready", "RELEASE_TAG": "runtime-v4.0.0"})
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Reusing verified public runtime release", result.stdout)

    def test_runtime_pack_consumes_the_explicit_buildctl_output(self):
        job = load_workflow(WORKFLOWS / "build_runtime_pack.yml")["jobs"]["build-runtime-pack"]
        download, = action_steps(job, "actions/download-artifact")
        self.assertEqual(download["with"], {
            "name": "runtime-editor-linux-x86_64",
            "path": "runtime-assets",
        })
        buildctl, = action_steps(job, "./.github/actions/setup-buildctl")
        pack = named_step(job, "Build runtime asset bundle")
        self.assertEqual(pack["env"]["BUILDCTL"], f"${{{{ steps.{buildctl['id']}.outputs.path }}}}")
        self.assertLess(job["steps"].index(buildctl), job["steps"].index(pack))

    def test_artifacts_cross_the_build_boundary_without_renaming(self):
        job = self.build_jobs["assemble-release"]
        download, = action_steps(job, "actions/download-artifact")
        # Minimatch excludes coverage, diagnostics and assembled outputs from retries.
        self.assertEqual(download["with"], {
            "pattern": "{runtime-*,web-release,web-worker-release,web-minigame-release,web-miniprogram-release,spx-standalone-*}",
            "path": "artifacts",
        })
        self.assertEqual(step_script(job, "Assemble immutable asset sets"),
                         "python3 .github/scripts/release/assemble.py")
        for upload, consumer in (("Upload assembled runtime release", "publish-runtime"),
                                 ("Upload assembled SPX release", "prepare-spx-release"),
                                 ("Upload assembled SPX release", "run-spxbench-suite")):
            producer_name = named_step(job, upload)["with"]["name"]
            download, = action_steps(self.jobs[consumer], "actions/download-artifact")
            self.assertEqual(download["with"]["name"], producer_name.replace("needs.plan-release.", "needs.build-release."))
        self.assertEqual(self.jobs["publish-npm"]["with"]["release_artifact"], "web-release")

    def test_candidate_validation_uses_the_assembled_web_archive(self):
        job = self.jobs["run-spxbench-suite"]
        download, = action_steps(job, "actions/download-artifact")
        validation = named_step(job, "Run spxbench regression suite")
        cleanup = named_step(job, "Retry candidate prerelease cleanup")
        self.assertLess(job["steps"].index(download), job["steps"].index(validation))
        self.assertLess(job["steps"].index(validation), job["steps"].index(cleanup))
        self.assertEqual(job["env"]["CANDIDATE_VERSION"],
                         "${{ needs.build-release.outputs.version }}-rc.${{ github.run_id }}.${{ github.run_attempt }}")
        self.assertEqual(validation["env"]["WEB_ARCHIVE_PATH"], f"{download['with']['path']}/spx_web.zip")
        self.assertEqual(validation["env"]["SPXBENCH_API_TOKEN"], "${{ secrets.SPXBENCH_API_TOKEN }}")
        for step in (validation, cleanup):
            self.assertEqual(step["env"]["GH_TOKEN"], "${{ github.token }}")
        self.assertEqual(job_condition(cleanup),
                         f"always()&&steps.{validation['id']}.outputs.prerelease_cleanup_pending=='true'")

    def test_prerelease_cleanup_retry_reports_result_without_failing_gate(self):
        script = step_script(self.jobs["run-spxbench-suite"], "Retry candidate prerelease cleanup")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            version = "0.0.1-rc.1.1"
            env = {"PATH": f"{root}:{os.environ['PATH']}", "CANDIDATE_VERSION": version}
            for exit_code, summary_name in ((0, "success"), (1, "failure"), (1, "missing/summary")):
                with self.subTest(exit_code=exit_code, summary=summary_name):
                    write_executable(root / "gh", f'#!/bin/sh\necho "$@" > gh-args\nexit {exit_code}\n')
                    summary = root / summary_name
                    result = run_shell(script, cwd=root, env={**env, "GITHUB_STEP_SUMMARY": str(summary)})
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual((root / "gh-args").read_text().strip(),
                                     f"release delete v{version} --cleanup-tag --yes")
                    self.assertEqual("::warning" in result.stdout, exit_code != 0)
                    if summary.parent == root:
                        expected = "failed" if exit_code else "deleted"
                        self.assertIn(f"Prerelease cleanup retry: **{expected}**", summary.read_text())
                    else:
                        self.assertIn("Could not write prerelease cleanup result", result.stdout)

    def test_internal_build_scheduling_preserves_optional_platforms_and_runtime_reuse(self):
        dependencies = {
            "plan-release": (), "check-source": ("plan-release",),
            "build-engine": ("plan-release", "check-source"),
            "reuse-engine": ("plan-release", "check-source"),
            "build-runtime-pack": ("plan-release", "build-engine", "reuse-engine"),
            "runtime-prerequisites": ("plan-release", "check-source", "build-engine", "reuse-engine", "build-runtime-pack"),
            "assemble-release": ("plan-release", "runtime-prerequisites", "package-web", "package-macos", "package-windows", "package-linux"),
        }
        godot = self.build_jobs["build-engine"]
        self.assertEqual(job_condition(godot), "needs.plan-release.outputs.runtime_state=='missing'&&needs.plan-release.outputs.engine_state=='build'")
        self.assertRegex(godot["uses"], r"^goplus/godot/\.github/workflows/release_builds\.yml@[0-9a-f]{40}$")
        for platform in ("web", "macos", "windows", "linux"):
            name = f"package-{platform}"
            dependencies[name] = ("plan-release", "runtime-prerequisites")
            self.assertEqual(job_condition(self.build_jobs[name]),
                             "!cancelled()&&needs.runtime-prerequisites.result=='success'&&"
                             f"contains(fromJSON(needs.plan-release.outputs.package_platforms),'{platform}')")
            self.assertEqual(self.build_jobs[name]["with"]["engine_artifacts"],
                             "${{ needs.plan-release.outputs.runtime_state == 'missing' }}")
        self.assertEqual(set(self.build_jobs), set(dependencies))
        for name, needs in dependencies.items():
            self.assertEqual(job_needs(self.build_jobs[name]), needs, name)

    def test_engine_reuse_keeps_existing_artifact_names_and_provenance(self):
        job = self.build_jobs["reuse-engine"]
        self.assertEqual(job_condition(job),
                         "needs.plan-release.outputs.runtime_state=='missing'&&needs.plan-release.outputs.engine_state=='reuse'")
        verify = named_step(job, "Download and verify source engine")
        self.assertEqual(verify["run"], "python3 .github/scripts/release/reuse_engine.py")
        self.assertEqual(verify["env"]["RUNTIME_MANIFEST"], "${{ needs.plan-release.outputs.engine_manifest }}")
        assemble = named_step(self.build_jobs["assemble-release"], "Assemble immutable asset sets")
        self.assertEqual(assemble["env"]["ENGINE_STATE"], "${{ needs.plan-release.outputs.engine_state }}")
        root = WORKFLOWS.parent.parent
        action = ".github/actions/upload-reused-engine"
        upload, = action_steps(load_workflow(root / action / "action.yml")["runs"], "actions/upload-artifact")
        self.assertEqual(upload["with"], {
            "name": "runtime-${{ inputs.artifact }}",
            "path": "reused-engine/${{ inputs.file || format('{0}.zip', inputs.artifact) }}",
            "if-no-files-found": "error", "retention-days": "14",
        })
        uploads = action_steps(job, "./" + action)
        lock = json.loads((root / "internal/release/runtime.lock.json").read_text())
        expected = [("macos-universal" if name == "macos.zip" else name.removesuffix(".zip"), name)
                    for name in lock["required_assets"] if name != "spx-runtime-assets.zip"]
        expected.append(("engine-provenance", "engine-source-manifest.json"))
        actual = []
        for step in uploads:
            artifact = step["with"]["artifact"]
            actual.append((artifact, step["with"].get("file", artifact + ".zip")))
            self.assertLess(job["steps"].index(verify), job["steps"].index(step))
        self.assertCountEqual(actual, expected)
        pack = self.build_jobs["build-runtime-pack"]
        self.assertEqual(job_condition(pack),
                         "!cancelled()&&needs.plan-release.outputs.runtime_state=='missing'&&"
                         "(needs.build-engine.result=='success'||needs.reuse-engine.result=='success')")
        resolve = named_step(self.build_jobs["plan-release"], "Resolve locked runtime release")
        self.assertEqual(resolve["env"]["REUSE_ENGINE_VERSION"], "${{ inputs.reuse_engine_version }}")
        self.assertIn('--reuse-engine-version "$REUSE_ENGINE_VERSION"', resolve["run"])

    def test_runtime_gate_requires_one_complete_engine_path(self):
        condition = job_condition(self.build_jobs["runtime-prerequisites"])
        statuses = ("success", "failure", "skipped", "cancelled")
        for state, plan, check, build, reuse, pack, cancelled in itertools.product(
                ("ready", "missing", "incompatible"), statuses, statuses,
                statuses, statuses, statuses, (True, False)):
            expression = condition.replace("!cancelled()", str(not cancelled))
            values = {"plan-release.outputs.runtime_state": state,
                      "plan-release.result": plan, "check-source.result": check,
                      "build-engine.result": build, "reuse-engine.result": reuse,
                      "build-runtime-pack.result": pack}
            for key, value in values.items():
                expression = expression.replace("needs." + key, repr(value))
            expression = expression.replace("&&", " and ").replace("||", " or ")
            expected = ((state == "ready" and build == reuse == pack == "skipped") or
                        (state == "missing" and pack == "success" and
                         (build, reuse) in (("success", "skipped"), ("skipped", "success"))))
            expected = expected and not cancelled and plan == check == "success"
            with self.subTest(inputs=values, cancelled=cancelled):
                self.assertEqual(eval(expression, {"__builtins__": {}}), expected)

    def test_publication_scheduling_and_development_isolation(self):
        contract = {
            "build-release": ((), "inputs.operation!='publish-dev-npm'"),
            "run-spxbench-suite": (("build-release",), "!cancelled()&&inputs.operation=='dry-run'&&needs.build-release.result=='success'"),
            "publish-runtime": (("build-release",), "!cancelled()&&inputs.operation!='dry-run'&&needs.build-release.result=='success'"),
            "prepare-spx-release": (("build-release", "publish-runtime"), "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-runtime.result=='success'"),
            "publish-npm": (("build-release", "prepare-spx-release"), "!cancelled()&&inputs.operation=='publish-release'&&needs.prepare-spx-release.result=='success'"),
            "publish-spx-release": (("build-release", "prepare-spx-release", "publish-npm"), "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-npm.result=='success'"),
            "validate-dev-npm": ((), "inputs.operation=='publish-dev-npm'"),
            "publish-dev-npm": (("validate-dev-npm",), "!cancelled()&&inputs.operation=='publish-dev-npm'&&needs.validate-dev-npm.result=='success'"),
        }
        self.assertEqual(set(self.jobs), {*contract, "release-gate"})
        for name, (needs, condition) in contract.items():
            self.assertEqual(job_needs(self.jobs[name]), needs, name)
            self.assertEqual(job_condition(self.jobs[name]), condition, name)

    def test_assembly_requires_successful_prerequisites_and_selected_packages(self):
        job = self.build_jobs["assemble-release"]
        self.assertEqual(job_condition(job), "!cancelled()")
        self.assertEqual(job["env"]["PACKAGE_PLATFORMS"], "${{ needs.plan-release.outputs.package_platforms }}")
        guard = job["steps"][0]
        self.assertNotIn("if", guard)
        self.assertEqual(guard["shell"], "python")
        self.assertEqual(guard["env"], {"NEEDS_JSON": "${{ toJSON(needs) }}"})

        def run_guard(needs, platforms):
            return run_command([sys.executable, "-c", guard["run"]], env={
                "NEEDS_JSON": json.dumps(needs), "PACKAGE_PLATFORMS": json.dumps(platforms),
            })

        platforms = ("web", "macos", "windows", "linux")
        for selected in ((), ("linux",), platforms):
            required = ("plan-release", "runtime-prerequisites", *(f"package-{p}" for p in selected))
            needs = {name: {"result": "success" if name in required else "skipped"} for name in job_needs(job)}
            self.assertEqual(run_guard(needs, selected).returncode, 0)
            for name, result in itertools.product(required, ("failure", "skipped", "cancelled", None)):
                with self.subTest(selected=selected, job=name, result=result):
                    failed = needs.copy()
                    if result is None:
                        del failed[name]
                    else:
                        failed[name] = {"result": result}
                    completed = run_guard(failed, selected)
                    self.assertNotEqual(completed.returncode, 0)
                    self.assertIn(name, completed.stderr)
        for invalid in (None, {}, ["unknown"]):
            with self.subTest(platforms=invalid):
                result = run_guard({name: {"result": "success"} for name in job_needs(job)}, invalid)
                self.assertNotEqual(result.returncode, 0)

    def test_release_gate_is_operation_aware_and_fail_closed(self):
        gate = self.jobs["release-gate"]
        required = {
            "dry-run": ("build-release", "run-spxbench-suite"),
            "publish-runtime": ("build-release", "publish-runtime"),
            "publish-release": ("build-release", "publish-runtime", "prepare-spx-release", "publish-npm", "publish-spx-release"),
            "publish-dev-npm": ("validate-dev-npm", "publish-dev-npm"),
        }
        terminal_jobs = (*required["publish-release"], "run-spxbench-suite", *required["publish-dev-npm"])
        self.assertEqual(job_needs(gate), terminal_jobs)
        self.assertEqual(job_condition(gate), "!cancelled()")
        step = named_step(gate, "Require the selected operation's terminal jobs")
        self.assertEqual(step["env"], {"OPERATION": "${{ inputs.operation }}", "NEEDS_JSON": "${{ toJSON(needs) }}"})
        self.assertEqual(step["shell"], "python")
        for operation, jobs in required.items():
            success = {job: {"result": "success" if job in jobs else "skipped"} for job in terminal_jobs}
            result = run_command([sys.executable, "-c", step["run"]], env={"OPERATION": operation, "NEEDS_JSON": json.dumps(success)})
            self.assertEqual(result.returncode, 0, result.stderr)
            for job, status in itertools.product(jobs, ("skipped", "failure", "cancelled", None)):
                with self.subTest(operation=operation, job=job, status=status):
                    failed = {**success, job: {"result": status}}
                    result = run_command([sys.executable, "-c", step["run"]], env={"OPERATION": operation, "NEEDS_JSON": json.dumps(failed)})
                    self.assertNotEqual(result.returncode, 0, result.stdout)
        result = run_command([sys.executable, "-c", step["run"]], env={"OPERATION": "unknown", "NEEDS_JSON": "{}"})
        self.assertNotEqual(result.returncode, 0, result.stdout)


class StandaloneWorkflowTest(unittest.TestCase):
    def test_platform_jobs_keep_their_native_boundaries(self):
        for platform, target in (("linux", "linux"), ("macos", "darwin"), ("windows", "windows")):
            job = load_workflow(WORKFLOWS / f"package_{platform}.yml")["jobs"][f"package-{platform}"]
            action, = action_steps(job, "./.github/actions/package-standalone")
            self.assertEqual(action["with"]["platform"], target)
            self.assertIn("engine-asset-dir", action["with"])
            self.assertIn("artifact-name", action["with"])
            if platform == "macos":
                self.assertEqual({row["arch_name"] for row in job["strategy"]["matrix"]["include"]}, {"x64", "arm64"})
            else:
                self.assertNotIn("strategy", job)


if __name__ == "__main__":
    unittest.main()
