#!/usr/bin/env python3

import itertools
import json
import re
import sys
import tempfile
import unittest

from test_support import (WORKFLOWS, action_steps, job_condition, job_needs,
                          load_workflow, named_step, run_command, run_shell, step_script)


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
        for name in ("publish-runtime", "prepare-spx-release", "publish-spx-release"):
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
                                       for name in ("operation", "release_tag", "platforms")})
        self.assertEqual(set(self.build_workflow["on"]), {"workflow_call"})
        self.assertEqual(self.build_workflow["permissions"], {"contents": "read"})
        self.assertNotIn("concurrency", self.build_workflow)
        self.assertTrue(all("permissions" not in job for job in self.build_jobs.values()))
        interface = self.build_workflow["on"]["workflow_call"]
        self.assertEqual(set(interface["inputs"]), set(call["with"]))
        self.assertTrue(all(spec["required"] == "true" and spec["type"] == "string"
                            for spec in interface["inputs"].values()))
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
        for platform in ("web", "macos", "windows", "linux"):
            self.assertEqual(planning["outputs"][f"run_{platform}"], "${{ steps." + plan["id"] + f".outputs.run_{platform}" + " }}")

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
                                 ("Upload assembled SPX release", "prepare-spx-release")):
            producer_name = named_step(job, upload)["with"]["name"]
            download, = action_steps(self.jobs[consumer], "actions/download-artifact")
            self.assertEqual(download["with"]["name"], producer_name.replace("needs.plan-release.", "needs.build-release."))
        self.assertEqual(self.jobs["publish-npm"]["with"]["release_artifact"], "web-release")

    def test_internal_build_scheduling_preserves_optional_platforms_and_runtime_reuse(self):
        conditions = {
            "runtime-prerequisites": (
                "!cancelled()&&needs.plan-release.result=='success'&&needs.check-source.result=='success'&&"
                "((needs.plan-release.outputs.runtime_state=='ready'&&needs.build-godot-runtime.result=='skipped'&&"
                "needs.build-runtime-pack.result=='skipped')||(needs.plan-release.outputs.runtime_state=='missing'&&"
                "needs.build-godot-runtime.result=='success'&&needs.build-runtime-pack.result=='success'))"
            ),
            "assemble-release": (
                "!cancelled()&&needs.plan-release.result=='success'&&needs.runtime-prerequisites.result=='success'&&"
                "(needs.plan-release.outputs.run_web!='true'||needs.package-web.result=='success')&&"
                "(needs.plan-release.outputs.run_macos!='true'||needs.package-macos.result=='success')&&"
                "(needs.plan-release.outputs.run_windows!='true'||needs.package-windows.result=='success')&&"
                "(needs.plan-release.outputs.run_linux!='true'||needs.package-linux.result=='success')"
            ),
        }
        dependencies = {
            "plan-release": (), "check-source": ("plan-release",),
            "build-godot-runtime": ("plan-release", "check-source"),
            "build-runtime-pack": ("build-godot-runtime",),
            "runtime-prerequisites": ("plan-release", "check-source", "build-godot-runtime", "build-runtime-pack"),
            "assemble-release": ("plan-release", "runtime-prerequisites", "package-web", "package-macos", "package-windows", "package-linux"),
            "build-gate": ("plan-release", "assemble-release"),
        }
        godot = self.build_jobs["build-godot-runtime"]
        self.assertEqual(job_condition(godot), "needs.plan-release.outputs.runtime_state=='missing'")
        self.assertRegex(godot["uses"], r"^goplus/godot/\.github/workflows/release_builds\.yml@[0-9a-f]{40}$")
        for platform in ("web", "macos", "windows", "linux"):
            name = f"package-{platform}"
            dependencies[name] = ("plan-release", "runtime-prerequisites")
            conditions[name] = ("!cancelled()&&needs.runtime-prerequisites.result=='success'&&"
                                f"needs.plan-release.outputs.run_{platform}=='true'")
            self.assertEqual(self.build_jobs[name]["with"]["engine_artifacts"],
                             "${{ needs.plan-release.outputs.runtime_state == 'missing' }}")
        self.assertEqual(set(self.build_jobs), set(dependencies))
        for name, needs in dependencies.items():
            self.assertEqual(job_needs(self.build_jobs[name]), needs, name)
        for name, condition in conditions.items():
            self.assertEqual(job_condition(self.build_jobs[name]), condition, name)

    def test_publication_scheduling_and_development_isolation(self):
        contract = {
            "build-release": ((), "inputs.operation!='publish-dev-npm'"),
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

    def test_build_completion_gate_rejects_missing_failed_and_skipped_assembly(self):
        gate = self.build_jobs["build-gate"]
        self.assertEqual(job_condition(gate), "!cancelled()")
        step = named_step(gate, "Require verified release artifacts")
        self.assertEqual(step["env"], {"PLAN_RESULT": "${{ needs.plan-release.result }}",
                                       "ASSEMBLE_RESULT": "${{ needs.assemble-release.result }}"})
        for plan, assemble in itertools.product(("success", "skipped", "failure", "cancelled", ""), repeat=2):
            with self.subTest(plan=plan, assemble=assemble):
                result = run_shell(step["run"], env={"PLAN_RESULT": plan, "ASSEMBLE_RESULT": assemble})
                self.assertEqual(result.returncode == 0, plan == assemble == "success", result.stderr)

    def test_release_gate_is_operation_aware_and_fail_closed(self):
        gate = self.jobs["release-gate"]
        required = {
            "dry-run": ("build-release",),
            "publish-runtime": ("build-release", "publish-runtime"),
            "publish-release": ("build-release", "publish-runtime", "prepare-spx-release", "publish-npm", "publish-spx-release"),
            "publish-dev-npm": ("validate-dev-npm", "publish-dev-npm"),
        }
        terminal_jobs = (*required["publish-release"], *required["publish-dev-npm"])
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
