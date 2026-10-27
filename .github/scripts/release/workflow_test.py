#!/usr/bin/env python3

import json
import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


WORKFLOW_PATH = Path(__file__).resolve().parents[2] / "workflows" / "release.yml"
RUNNER_WORKFLOW_PATH = Path(__file__).resolve().parents[2] / "workflows" / "runner.yml"
STATIC_CHECKS_WORKFLOW_PATH = (
    Path(__file__).resolve().parents[2] / "workflows" / "static_checks.yml"
)
ASSEMBLE_SCRIPT_PATH = Path(__file__).resolve().parent / "assemble.sh"
GO_TEST_SCRIPT_PATH = Path(__file__).resolve().parents[1] / "test_go.sh"
GO_COVERAGE_SCRIPT_PATH = Path(__file__).resolve().parents[1] / "test_go_coverage.sh"
GO_BUILDS_WORKFLOW_PATH = WORKFLOW_PATH.with_name("go_builds.yml")
WEB_BUILD_WORKFLOW_PATH = WORKFLOW_PATH.with_name("build_web.yml")
WEB_PACKAGE_WORKFLOW_PATH = (
    Path(__file__).resolve().parents[2] / "workflows" / "publish_web_package.yml"
)


def run_command(command, *, cwd=None, env=None, check=False):
    return subprocess.run(
        command,
        cwd=cwd,
        env={**os.environ, **(env or {})},
        capture_output=True,
        text=True,
        check=check,
    )


def run_shell(script, *, cwd=None, env=None):
    return run_command(["bash", "-euo", "pipefail", "-c", script], cwd=cwd, env=env)


def write_executable(path, content):
    path.write_text(content, encoding="utf-8")
    path.chmod(0o755)


def job_blocks(text):
    lines = text.splitlines()
    try:
        jobs_line = lines.index("jobs:")
    except ValueError as exc:
        raise AssertionError("release workflow has no top-level jobs mapping") from exc

    starts = [
        (index, match.group(1))
        for index, line in enumerate(lines[jobs_line + 1 :], jobs_line + 1)
        if (match := re.fullmatch(r"  ([a-z0-9-]+):", line))
    ]
    return {
        name: "\n".join(lines[start : starts[position + 1][0]])
        if position + 1 < len(starts)
        else "\n".join(lines[start:])
        for position, (start, name) in enumerate(starts)
    }


def job_needs(block):
    lines = block.splitlines()
    for index, line in enumerate(lines):
        if match := re.fullmatch(r"    needs: \[([^]]*)\]", line):
            return tuple(value.strip() for value in match.group(1).split(","))
        if match := re.fullmatch(r"    needs: ([a-z0-9-]+)", line):
            return (match.group(1),)
        if line == "    needs:":
            values = []
            for item in lines[index + 1 :]:
                if not (match := re.fullmatch(r"      - ([a-z0-9-]+)", item)):
                    break
                values.append(match.group(1))
            return tuple(values)
    return ()


def job_condition(block):
    lines = block.splitlines()
    for index, line in enumerate(lines):
        if not (match := re.fullmatch(r"    if: ?(.*)", line)):
            continue
        if match.group(1) not in (">-", "|", ""):
            value = match.group(1)
        else:
            parts = []
            for item in lines[index + 1 :]:
                if not item.startswith("      "):
                    break
                parts.append(item.strip())
            value = " ".join(parts)
        return "".join(value.replace("${{", "").replace("}}", "").split())
    return ""


def step_script(block, name):
    lines = block.splitlines()
    try:
        start = next(index for index, line in enumerate(lines) if line.lstrip() == f"- name: {name}")
        indent = len(lines[start]) - len(lines[start].lstrip()) + 2
        end = next(
            (index for index in range(start + 1, len(lines))
             if lines[index].strip() and not lines[index].startswith(" " * indent)),
            len(lines),
        )
        run_line = lines.index(" " * indent + "run: |", start + 1, end)
    except (StopIteration, ValueError) as exc:
        raise AssertionError(f"missing literal run script for step {name!r}") from exc

    body = []
    for line in lines[run_line + 1 : end]:
        if line and not line.startswith(" " * (indent + 2)):
            break
        body.append(line[indent + 2 :] if line else line)
    return "\n".join(body)


def workflow_input_block(text, name):
    lines = text.splitlines()
    try:
        start = lines.index(f"      {name}:")
    except ValueError as exc:
        raise AssertionError(f"missing workflow input {name!r}") from exc

    body = [lines[start]]
    for line in lines[start + 1 :]:
        if re.fullmatch(r"      [a-z0-9_]+:", line):
            break
        if line and not line.startswith("        "):
            break
        body.append(line)
    return "\n".join(body)


class ReleaseWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = WORKFLOW_PATH.read_text(encoding="utf-8")
        cls.jobs = job_blocks(cls.workflow)
        cls.runner_workflow = RUNNER_WORKFLOW_PATH.read_text(encoding="utf-8")
        cls.runner_jobs = job_blocks(cls.runner_workflow)
        cls.static_checks_workflow = STATIC_CHECKS_WORKFLOW_PATH.read_text(encoding="utf-8")
        cls.static_checks_jobs = job_blocks(cls.static_checks_workflow)
        cls.web_package_workflow = WEB_PACKAGE_WORKFLOW_PATH.read_text(encoding="utf-8")
        cls.web_package_jobs = job_blocks(cls.web_package_workflow)

    def test_export_types_snapshot_is_checked_at_release(self):
        input_block = workflow_input_block(self.static_checks_workflow, "check_export_types")
        self.assertIn("        type: boolean", input_block)
        self.assertIn("        default: false", input_block)
        self.assertIn("      check_export_types: true", self.jobs["static-checks"])
        self.assertNotIn("check_export_types: true", self.runner_jobs["static-checks"])

        steps = self.static_checks_jobs["static-checks"]
        restore = (
            "      - name: Restore committed SPX type snapshot in regular CI\n"
            "        if: ${{ !inputs.check_export_types }}\n"
            "        run: git restore -- pkg/ispx/internal/pkg/github.com/goplus/spx/v4/export.types"
        )
        check = "      - name: Ensure generated files are up to date\n        run: git diff --exit-code"
        self.assertIn(restore, steps)
        self.assertIn(check, steps)
        self.assertLess(steps.index(restore), steps.index(check))

    def test_publish_dev_input_contract(self):
        release_tag = workflow_input_block(self.workflow, "release_tag")
        self.assertIn("        required: false", release_tag)
        self.assertIn("        default: ''", release_tag)

        operation = workflow_input_block(self.workflow, "operation")
        self.assertIn("          - publish-dev-npm", operation)

    def test_runner_selects_current_build_for_an_incompatible_runtime(self):
        resolution = self.runner_jobs["runtime-resolution"]
        self.assertIn("            --repo-root . \\\n", resolution)
        self.assertIn("            --revision HEAD \\\n", resolution)

        self.assertEqual(
            job_condition(self.runner_jobs["spx-module-integration"]),
            "needs.runtime-resolution.outputs.runtime_state=='missing'||"
            "needs.runtime-resolution.outputs.runtime_state=='incompatible'",
        )
        self.assertEqual(
            job_condition(self.runner_jobs["web-build"]),
            "needs.runtime-resolution.outputs.runtime_state=='ready'",
        )

        gate = step_script(
            self.runner_jobs["ci-gate"], "Verify required CI path passed"
        )
        base_env = {
            "STATIC_CHECKS_RESULT": "success",
            "GO_BUILDS_RESULT": "success",
            "COVERAGE_RESULT": "success",
            "RUNTIME_RESOLUTION_RESULT": "success",
            "RUNTIME_REASON": "test runtime resolution",
        }
        for state in ("ready", "missing", "incompatible"):
            expected = {
                **base_env, "RUNTIME_STATE": state,
                "MODULE_INTEGRATION_RESULT": "skipped" if state == "ready" else "success",
                "WEB_BUILD_RESULT": "success" if state == "ready" else "skipped",
            }
            with self.subTest(state=state):
                completed = run_shell(gate, env=expected)
                self.assertEqual(completed.returncode, 0, completed.stderr)
            for job in (
                "STATIC_CHECKS_RESULT", "GO_BUILDS_RESULT", "COVERAGE_RESULT", "RUNTIME_RESOLUTION_RESULT",
                "MODULE_INTEGRATION_RESULT", "WEB_BUILD_RESULT",
            ):
                for result in ("success", "failure", "cancelled", "skipped", ""):
                    if result == expected[job]:
                        continue
                    with self.subTest(state=state, job=job, result=result):
                        completed = run_shell(gate, env={**expected, job: result})
                        self.assertNotEqual(completed.returncode, 0)
                        self.assertIn("want " + expected[job], completed.stderr)
        for state in ("unknown", ""):
            with self.subTest(state=state):
                completed = run_shell(gate, env={
                    **base_env, "RUNTIME_STATE": state,
                    "MODULE_INTEGRATION_RESULT": "success", "WEB_BUILD_RESULT": "skipped",
                })
                self.assertNotEqual(completed.returncode, 0)
                self.assertIn("Unknown runtime state", completed.stderr)

    def test_release_platform_selection_preserves_output_order_and_validation(self):
        script = step_script(self.jobs["setup"], "Resolve SPX package targets")
        cases = (
            ("dry-run", "all", ("web", "macos", "windows", "linux")),
            ("dry-run", " WINDOWS , web ", ("web", "windows")),
            ("dry-run", "linux", ("linux",)),
            ("dry-run", "web,web", "Duplicate platform: web"),
            ("dry-run", "other", "Unsupported platform: other"),
            ("dry-run", "", "Platforms must not be empty"),
            *(("dry-run", platforms, "Platforms must not contain empty values")
              for platforms in (",", ",web", "web,,linux", "web,", "web, ,linux")),
            ("publish-release", "all", ("web", "macos", "windows", "linux")),
            ("publish-release", "web", "Publishing requires platforms=all"),
            ("publish-runtime", "invalid,invalid", ()),
        )
        for operation, platforms, expected in cases:
            with self.subTest(operation=operation, platforms=platforms), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                result = run_shell(script, env={
                    "OPERATION_INPUT": operation, "PLATFORMS_INPUT": platforms,
                    "GITHUB_OUTPUT": str(output),
                })
                if isinstance(expected, str):
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(expected, result.stderr)
                    self.assertFalse(output.exists())
                else:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(output.read_text().splitlines(), [
                        f"run_{platform}={'true' if platform in expected else 'false'}"
                        for platform in ("web", "macos", "windows", "linux")
                    ])

    def test_release_rejects_an_incompatible_runtime_with_bump_guidance(self):
        setup = self.jobs["setup"]
        self.assertIn("            --repo-root . \\\n", setup)
        self.assertIn("            --revision HEAD \\\n", setup)
        self.assertIn(
            "steps.runtime.outputs.runtime_state != 'incompatible'",
            setup,
        )

        guard = step_script(setup, "Enforce runtime reuse policy")
        cases = (
            ("ready", True),
            ("missing", True),
            ("incompatible", False),
            ("unknown", False),
        )
        for runtime_state, succeeds in cases:
            with self.subTest(runtime_state=runtime_state):
                completed = run_shell(
                    guard,
                    env={
                        "RUNTIME_STATE": runtime_state,
                        "RUNTIME_REASON": "runtime-v1.2.3 cannot be reused; bump runtime_version",
                    },
                )
                self.assertEqual(
                    completed.returncode == 0,
                    succeeds,
                    completed.stderr,
                )
                if runtime_state == "incompatible":
                    self.assertIn("Runtime version bump required", completed.stdout)
                    self.assertIn("make bump-release", completed.stderr)
                    self.assertIn("bump runtime_version", completed.stderr)

        dev_guard = self.jobs["dev-npm-guard"]
        self.assertIn("      - name: Resolve development runtime release", dev_guard)
        self.assertIn("            --repo-root . \\\n", dev_guard)
        self.assertIn("            --revision HEAD \\\n", dev_guard)
        policy = step_script(dev_guard, "Require reusable development runtime")
        for runtime_state, succeeds in (
            ("ready", True),
            ("missing", False),
            ("incompatible", False),
            ("unknown", False),
        ):
            with self.subTest(dev_runtime_state=runtime_state):
                completed = run_shell(
                    policy,
                    env={
                        "RUNTIME_STATE": runtime_state,
                        "RUNTIME_REASON": "runtime-v1.2.3 cannot be reused; bump runtime_version",
                    },
                )
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)
                if runtime_state == "missing":
                    self.assertIn("Published runtime required", completed.stdout)
                    self.assertIn("Publish the declared runtime", completed.stderr)
                if runtime_state == "incompatible":
                    self.assertIn("Runtime version bump required", completed.stdout)
                    self.assertIn("make bump-release", completed.stderr)

    def test_job_scheduling_contract(self):
        needs = {
            "setup": (),
            "runtime-ready": ("setup", "static-checks", "godot-runtime-build", "runtime-assets-build"),
            "assemble": ("setup", "runtime-ready", "web-build", "macos-build", "windows-build", "linux-build"),
            "publish-runtime": ("setup", "assemble"),
            "publish-spx": ("setup", "assemble", "publish-runtime"),
            "publish-web-package": ("setup", "publish-spx"),
            "finalize-spx": ("setup", "publish-spx", "publish-web-package"),
            "dev-npm-guard": (),
            "publish-dev-web-package": ("dev-npm-guard",),
        }
        conditions = {
            "setup": "inputs.operation!='publish-dev-npm'",
            "runtime-ready": (
                "!cancelled()&&needs.setup.result=='success'&&needs.static-checks.result=='success'&&"
                "((needs.setup.outputs.runtime_state=='ready'&&needs.godot-runtime-build.result=='skipped'&&"
                "needs.runtime-assets-build.result=='skipped')||(needs.setup.outputs.runtime_state=='missing'&&"
                "needs.godot-runtime-build.result=='success'&&needs.runtime-assets-build.result=='success'))"
            ),
            "assemble": (
                "!cancelled()&&needs.setup.result=='success'&&needs.runtime-ready.result=='success'&&"
                "(needs.setup.outputs.run_web!='true'||needs.web-build.result=='success')&&"
                "(needs.setup.outputs.run_macos!='true'||needs.macos-build.result=='success')&&"
                "(needs.setup.outputs.run_windows!='true'||needs.windows-build.result=='success')&&"
                "(needs.setup.outputs.run_linux!='true'||needs.linux-build.result=='success')"
            ),
            "publish-runtime": "!cancelled()&&inputs.operation!='dry-run'&&needs.assemble.result=='success'",
            "publish-spx": "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-runtime.result=='success'",
            "publish-web-package": "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-spx.result=='success'",
            "finalize-spx": "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-web-package.result=='success'",
            "dev-npm-guard": "inputs.operation=='publish-dev-npm'",
            "publish-dev-web-package": (
                "!cancelled()&&inputs.operation=='publish-dev-npm'&&"
                "needs.dev-npm-guard.result=='success'"
            ),
        }
        for platform in ("web", "macos", "windows", "linux"):
            job = f"{platform}-build"
            needs[job] = ("setup", "runtime-ready")
            conditions[job] = (
                "!cancelled()&&needs.runtime-ready.result=='success'&&"
                f"needs.setup.outputs.run_{platform}=='true'"
            )

        for job, expected in conditions.items():
            with self.subTest(job=job):
                self.assertEqual(job_needs(self.jobs[job]), needs[job])
                self.assertEqual(job_condition(self.jobs[job]), expected)

    def test_assemble_implementation_is_kept_out_of_the_workflow_dag(self):
        block = self.jobs["assemble"]
        self.assertIn(
            "        run: bash .github/scripts/release/assemble.sh",
            block,
        )
        self.assertIn(
            "if: needs.setup.outputs.runtime_state == 'missing' || inputs.operation == 'dry-run'",
            block,
        )
        script = ASSEMBLE_SCRIPT_PATH.read_text(encoding="utf-8")
        self.assertIn('echo "product_count=$product_count" >> "$GITHUB_OUTPUT"', script)
        self.assertIn("dist/runtime", script)
        self.assertIn("dist/product", script)

    def test_runtime_publish_reuses_the_selected_version(self):
        block = self.jobs["publish-runtime"]
        script = step_script(block, "Publish immutable runtime release")

        self.assertIn("    name: Publish or reuse runtime release", block)
        self.assertIn(
            "      - name: Check out release source for the publish guard\n"
            "        if: needs.setup.outputs.runtime_state == 'missing'",
            block,
        )
        self.assertIn(
            "      - name: Download assembled runtime release\n"
            "        if: needs.setup.outputs.runtime_state == 'missing'",
            block,
        )
        self.assertIn(
            "          RUNTIME_STATE: ${{ needs.setup.outputs.runtime_state }}",
            block,
        )
        self.assertIn('if [ "$RUNTIME_STATE" = ready ]; then', script)
        self.assertIn("Reusing verified public runtime release", script)
        self.assertLess(
            script.index('if [ "$RUNTIME_STATE" = ready ]; then'),
            script.index('gh release view "$RELEASE_TAG"'),
        )
        self.assertIn('--verify-manifest "published/$RUNTIME_MANIFEST"', script)
        self.assertIn('--asset-directory published', script)
        self.assertIn("sha256sum -c SHA256SUMS", script)
        self.assertIn("diff -u", script)
        self.assertIn("Reusing public runtime release for $RUNTIME_VERSION", script)
        self.assertNotIn('cmp -s "dist/$RUNTIME_MANIFEST"', script)

    def test_publish_steps_reuse_metadata_and_stop_before_upload_on_failure(self):
        mocks = r'''gh() {
  local operation="$2"
  printf 'gh %s\n' "$operation" >> "$TRACE"
  [ "$operation" != "$FAIL_OPERATION" ] || return 7
  case "$operation" in
    view)
      [[ "$*" == *'--json isDraft,assets' ]] || return 98
      [ "$RELEASE_STATE" != absent ] || return 1
      printf '%s\n' "$RELEASE_JSON"
      ;;
    download) cp remote/* published/ ;;
    create|delete-asset|upload|edit) ;;
    *) return 99 ;;
  esac
}
sha256sum() { echo checksum >> "$TRACE"; return "$CHECKSUM_EXIT"; }
go() { echo manifest >> "$TRACE"; return "$MANIFEST_EXIT"; }
# The workflow uses GNU find; keep this fixture portable to macOS.
find() {
  local path
  for path in "$1"/*; do
    [ ! -f "$path" ] || printf '%s\n' "${path##*/}"
  done
}
'''
        for job, step in (
            ("publish-runtime", "Publish immutable runtime release"),
            ("publish-spx", "Prepare SPX release draft"),
        ):
            scenarios = ["absent", "draft", "public", "extra-assets", "download-failure",
                         "checksum-failure", "create-failure"]
            if job == "publish-runtime":
                scenarios.append("manifest-failure")
            for scenario in scenarios:
                with self.subTest(job=job, scenario=scenario), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    for folder in ("dist", "remote"):
                        (root / folder).mkdir()
                        for name in ("asset.zip", "manifest.json", "SHA256SUMS"):
                            (root / folder / name).write_text("same fixture contents\n")
                    if scenario == "extra-assets":
                        (root / "remote" / "extra.zip").write_text("unexpected\n")
                    state = scenario if scenario in ("absent", "draft") else "public"
                    if scenario == "create-failure":
                        state = "absent"
                    metadata = {"isDraft": state == "draft", "assets": [
                        {"name": path.name} for path in sorted((root / "remote").iterdir())
                    ]}
                    result = run_shell(mocks + step_script(self.jobs[job], step), cwd=root, env={
                        "TRACE": str(root / "trace"), "RELEASE_STATE": state,
                        "RELEASE_JSON": json.dumps(metadata), "REPOSITORY": "fixture/spx",
                        "RELEASE_TAG": "v4.0.0", "GITHUB_SHA": "a" * 40,
                        "RUNTIME_STATE": "missing", "RUNTIME_VERSION": "4.0.0",
                        "RUNTIME_MANIFEST": "manifest.json",
                        "FAIL_OPERATION": scenario.removesuffix("-failure")
                        if scenario in ("download-failure", "create-failure") else "",
                        "CHECKSUM_EXIT": "7" if scenario == "checksum-failure" else "0",
                        "MANIFEST_EXIT": "7" if scenario == "manifest-failure" else "0",
                    })
                    self.assertEqual(result.returncode == 0, scenario in ("absent", "draft", "public"),
                                     result.stderr)
                    trace = (root / "trace").read_text().splitlines()
                    self.assertEqual(trace.count("gh view"), 1)
                    writes = [event for event in trace if event in (
                        "gh create", "gh delete-asset", "gh upload", "gh edit",
                    )]
                    expected = []
                    if scenario in ("absent", "draft"):
                        expected = (["gh create"] if scenario == "absent" else
                                    ["gh delete-asset"] * len(metadata["assets"]))
                        expected += ["gh upload"] + (["gh edit"] if job == "publish-runtime" else [])
                    elif scenario == "create-failure":
                        expected = ["gh create"]
                    self.assertEqual(writes, expected)
                    if scenario == "public":
                        self.assertIn("checksum", trace)
                        self.assertEqual("manifest" in trace, job == "publish-runtime")

    def test_dry_run_cannot_enter_any_publication_job(self):
        required_guards = {
            "publish-runtime": "inputs.operation!='dry-run'",
            "publish-spx": "inputs.operation=='publish-release'",
            "publish-web-package": "inputs.operation=='publish-release'",
            "finalize-spx": "inputs.operation=='publish-release'",
            "publish-dev-web-package": "inputs.operation=='publish-dev-npm'",
        }
        for job, guard in required_guards.items():
            with self.subTest(job=job):
                condition = job_condition(self.jobs[job])
                self.assertIn(guard, condition.split("&&"))
                self.assertNotIn("||", condition)

    def test_publish_dev_uses_exact_sha_and_oidc(self):
        block = self.jobs["publish-dev-web-package"]
        self.assertIn("    uses: ./.github/workflows/publish_web_package.yml", block)
        self.assertIn("      target: ${{ github.sha }}", block)
        for name in ("release_version", "release_artifact"):
            self.assertIn("        default: ''", workflow_input_block(self.web_package_workflow, name))
        self.assertIn("      contents: read", block)
        self.assertIn("      id-token: write", block)

        guard = step_script(
            self.jobs["dev-npm-guard"], "Require the canonical dev branch"
        )
        cases = (
            ("goplus/spx", "refs/heads/dev", "", True),
            ("someone/spx", "refs/heads/dev", "", False),
            ("goplus/spx", "refs/heads/release/v4.0.0", "", False),
            ("goplus/spx", "refs/heads/dev", "v4.0.0", False),
        )
        for repository, ref, release_tag, succeeds in cases:
            with self.subTest(repository=repository, ref=ref, release_tag=release_tag):
                completed = run_shell(
                    guard,
                    cwd=WORKFLOW_PATH.parents[2],
                    env={
                        "CURRENT_REF": ref,
                        "CURRENT_REPOSITORY": repository,
                        "RELEASE_TAG_INPUT": release_tag,
                    },
                )
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)

    def test_web_package_is_reusable_only_and_rejects_release_tags(self):
        self.assertNotIn("  workflow_dispatch:", self.web_package_workflow)
        self.assertIn(
            "permissions:\n  contents: read\n  id-token: write",
            self.web_package_workflow,
        )
        block = self.web_package_jobs["build"]

        validate_target = step_script(block, "Validate publication target")
        for target, release_version, release_artifact, succeeds in (
            ("a" * 40, "", "", True),
            ("a" * 40, "4.0.0", "web-release", True),
            ("a" * 40, "4.0.0", "", False),
            ("a" * 40, "", "web-release", False),
            ("dev", "", "", False),
            ("refs/heads/dev", "", "", False),
            ("A" * 40, "", "", False),
        ):
            with self.subTest(
                target=target,
                release_version=release_version,
                release_artifact=release_artifact,
            ):
                completed = run_shell(
                    validate_target,
                    env={
                        "RELEASE_ARTIFACT": release_artifact,
                        "RELEASE_VERSION": release_version,
                        "TARGET": target,
                    },
                )
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)

        verify_source = step_script(block, "Verify checked-out publication source")
        with tempfile.TemporaryDirectory() as repository:
            for command in (
                ("git", "init", "--quiet"),
                ("git", "config", "user.name", "SPX release test"),
                ("git", "config", "user.email", "release-test@example.invalid"),
                ("git", "config", "commit.gpgSign", "false"),
                ("git", "config", "tag.gpgSign", "false"),
                ("git", "commit", "--allow-empty", "--quiet", "-m", "test"),
                ("git", "tag", "v4.0.0"),
            ):
                subprocess.run(command, cwd=repository, check=True)
            target = run_command(
                ("git", "rev-parse", "HEAD"),
                cwd=repository,
                check=True,
            ).stdout.strip()

            tagged = run_shell(
                verify_source,
                cwd=repository,
                env={
                    "IS_DEVELOPMENT": "true",
                    "TARGET": target,
                },
            )
            self.assertNotEqual(tagged.returncode, 0, tagged.stdout)

            wrong_sha = run_shell(
                verify_source,
                cwd=repository,
                env={
                    "IS_DEVELOPMENT": "false",
                    "TARGET": "b" * 40,
                },
            )
            self.assertNotEqual(wrong_sha.returncode, 0, wrong_sha.stdout)
            self.assertIn("instead of publication target", wrong_sha.stderr)

            subprocess.run(
                ("git", "tag", "--delete", "v4.0.0"),
                cwd=repository,
                capture_output=True,
                check=True,
            )
            untagged = run_shell(
                verify_source,
                cwd=repository,
                env={
                    "IS_DEVELOPMENT": "true",
                    "TARGET": target,
                },
            )
            self.assertEqual(untagged.returncode, 0, untagged.stderr)

        publish = step_script(block, "Publish web package to npm")
        with tempfile.TemporaryDirectory() as workspace:
            workspace_path = Path(workspace)
            npm_scripts_path = workspace_path / ".github" / "scripts" / "npm"
            npm_scripts_path.mkdir(parents=True)
            write_executable(npm_scripts_path / "prepare_package.sh", "#!/bin/sh\nexit 0\n")

            fake_bin = workspace_path / "fake-bin"
            fake_bin.mkdir()
            write_executable(
                fake_bin / "npm",
                """#!/bin/sh
set -eu
printf '%s\\n' "$*" >> "$MOCK_NPM_TRACE"
case "$1:$3" in
  pack:*)
    printf '%s\\n' '[{"filename":"spx.tgz","integrity":"sha512-local"}]'
    ;;
  view:dist.integrity)
    [ "$MOCK_EXISTS" = true ] || exit 1
    printf '%s\\n' "$MOCK_INTEGRITY"
    ;;
  view:dist-tags)
    printf '{"dev":"%s"}\\n' "$MOCK_DIST_TAG"
    ;;
  publish:*) ;;
  *)
    exit 99
    ;;
esac
""",
            )

            version = "4.0.1-0.20260814000000-aaaaaaaaaaaa"
            trace = workspace_path / "npm-trace"
            for exists, integrity, remote_dist_tag, error in (
                (True, "sha512-local", version, ""),
                (True, "sha512-local", "older", "not selected by dist-tag"),
                (True, "sha512-different", version, "does not match"),
                (False, "", "", ""),
            ):
                with self.subTest(exists=exists, integrity=integrity, remote_dist_tag=remote_dist_tag):
                    trace.unlink(missing_ok=True)
                    completed = run_shell(
                        publish,
                        cwd=workspace,
                        env={
                            "IS_RELEASE": "false",
                            "MOCK_EXISTS": str(exists).lower(),
                            "MOCK_INTEGRITY": integrity,
                            "MOCK_DIST_TAG": remote_dist_tag,
                            "MOCK_NPM_TRACE": str(trace),
                            "PATH": f"{fake_bin}:{os.environ['PATH']}",
                            "SPX_VERSION": version,
                        },
                    )
                    self.assertEqual(
                        completed.returncode == 0,
                        not error,
                        completed.stderr,
                    )
                    if error:
                        self.assertIn(error, completed.stderr)
                    commands = trace.read_text().splitlines()
                    self.assertEqual(commands.count(f"view @xgo-pkgs/spx@{version} dist.integrity"), 1)
                    self.assertEqual([command for command in commands if command.startswith("publish ")],
                                     [] if exists else ["publish .npm/packages/spx.tgz --access public --tag dev"])

    def test_web_package_normalizes_versions_without_go_for_releases(self):
        script = step_script(self.web_package_jobs["build"], "Compute package version")
        trace = 'go() { echo "$*" >&2; echo v4.0.1-0.20260814000000-aaaaaaaaaaaa; return "${GO_EXIT:-0}"; }\n' + script
        for release, version in (("", "4.0.1-0.20260814000000-aaaaaaaaaaaa"),
                                 ("4.0.0", "4.0.0"), ("v4.0.0", "4.0.0")):
            with self.subTest(release=release), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "env"
                result = run_shell(trace, env={"RELEASE_VERSION": release, "GITHUB_ENV": str(output)})
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(output.read_text(), f"SPX_VERSION={version}\n")
                self.assertEqual(result.stderr, "" if release else "run ./.github/scripts/npm/pseudo_version.go HEAD\n")
                if not release:
                    output.unlink()
                    result = run_shell(trace, env={
                        "RELEASE_VERSION": release, "GITHUB_ENV": str(output), "GO_EXIT": "7",
                    })
                    self.assertEqual(result.returncode, 7, result.stderr)
                    self.assertFalse(output.exists())

    def test_release_gate_is_operation_aware_and_fail_closed(self):
        block = self.jobs["release-gate"]
        terminal_jobs = (
            "setup",
            "assemble",
            "publish-runtime",
            "publish-spx",
            "publish-web-package",
            "finalize-spx",
            "dev-npm-guard",
            "publish-dev-web-package",
        )
        self.assertEqual(job_needs(block), terminal_jobs)
        self.assertEqual(job_condition(block), "!cancelled()")
        self.assertIn("          OPERATION: ${{ inputs.operation }}", block)
        self.assertIn("          NEEDS_JSON: ${{ toJSON(needs) }}", block)

        script = step_script(block, "Require the selected operation's terminal jobs")
        cases = (
            ("dry-run", {"setup": "success", "assemble": "success"}, True),
            ("publish-runtime", {"setup": "success", "assemble": "success", "publish-runtime": "success"}, True),
            ("publish-release", {job: "success" for job in terminal_jobs[:6]}, True),
            ("publish-dev-npm", {"dev-npm-guard": "success", "publish-dev-web-package": "success"}, True),
            ("dry-run", {"setup": "success", "assemble": "skipped"}, False),
            ("publish-runtime", {"setup": "success", "assemble": "success", "publish-runtime": "skipped"}, False),
            ("publish-release", {"setup": "success", "assemble": "success", "publish-runtime": "success", "publish-spx": "success", "publish-web-package": "skipped"}, False),
            ("publish-dev-npm", {"dev-npm-guard": "skipped", "publish-dev-web-package": "skipped"}, False),
            ("publish-dev-npm", {"dev-npm-guard": "success", "publish-dev-web-package": "failure"}, False),
            ("unknown", {}, False),
        )
        for operation, results, succeeds in cases:
            with self.subTest(operation=operation, results=results):
                needs_json = {
                    job: {"result": results.get(job, "skipped")}
                    for job in terminal_jobs
                }
                completed = run_shell(
                    script,
                    env={
                        "OPERATION": operation,
                        "NEEDS_JSON": json.dumps(needs_json),
                    },
                )
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)


class StepScriptTest(unittest.TestCase):
    def test_extraction_stays_within_the_named_step(self):
        for indent in ("    ", "      "):
            for target in ("run: |\n  echo target", "uses: example/action@v1", "run: echo target"):
                with self.subTest(indent=len(indent), target=target):
                    block = (
                        f"{indent}- name: Target\n"
                        + "\n".join(indent + "  " + line for line in target.splitlines())
                        + f"\n{indent}- id: other\n{indent}  run: |\n{indent}    echo other"
                    )
                    if target.startswith("run: |"):
                        self.assertEqual(step_script(block, "Target"), "echo target")
                    else:
                        with self.assertRaisesRegex(AssertionError, "missing literal run script"):
                            step_script(block, "Target")


class StaticChecksWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow = STATIC_CHECKS_WORKFLOW_PATH.read_text(encoding="utf-8")
        cls.steps = job_blocks(workflow)["static-checks"]

    def test_workflow_preserves_test_stages_and_uploads_suite_outputs(self):
        steps = self.steps
        blocks = {
            block.splitlines()[0]: block
            for block in steps.split("      - name: ")[1:]
        }
        names = [
            "Test Web bridge",
            "Test Go Web packages",
            "Test Go Web process fallbacks",
            "Test Go Web interpreter",
            "Test Web ABI memory ownership",
            "Test CI tooling",
            "Test binding generator",
            "Test CI contracts",
            "Check runtime fingerprints",
            "Test Go modules",
            "Save coverage for combined upload",
        ]
        positions = [steps.index(f"- name: {name}\n") for name in names]
        self.assertEqual(positions, sorted(positions))
        suites = (
            ("web", "web-tests", "Test Go Web packages"),
            ("web-process", "web-process-tests", "Test Go Web process fallbacks"),
            ("web-interpreter", "web-interpreter-tests", "Test Go Web interpreter"),
            ("tools", "tool-tests", "Test CI tooling"),
            ("codegen", "codegen-tests", "Test binding generator"),
            ("native", "go-tests", "Test Go modules"),
        )
        for suite, step_id, name in suites:
            with self.subTest(suite=suite):
                block = blocks[name]
                self.assertIn(f"        id: {step_id}\n", block)
                self.assertIn(f"run: bash .github/scripts/test_go_coverage.sh {suite}", block)
                self.assertNotIn("working-directory:", block)
                self.assertNotIn("GOOS:", block)
                self.assertNotIn("GOARCH:", block)
        self.assertIn("node --test cmd/ispx/tests/*.test.cjs godot_modules/spx/web/tests/*.test.cjs", blocks[names[0]])
        self.assertIn("GDSPX_SANITIZERS: address,undefined", blocks[names[4]])
        self.assertIn("bash godot_modules/spx/web/tests/run_string_abi_test.sh", blocks[names[4]])
        self.assertIn(".github/scripts/test_go_coverage.sh", blocks["Check shell scripts"])
        self.assertIn("cmd/ispx/build.sh", blocks["Check shell scripts"])
        # Filenames belong to the runner; the workflow only combines its outputs.
        self.assertNotRegex(steps, r"coverage(?:-[\w-]+)?\.out")
        artifact = blocks["Save coverage for combined upload"]
        output_steps = re.findall(r"\$\{\{\s*steps\.([a-z-]+)\.outputs\.coverage\s*\}\}", artifact)
        self.assertEqual(output_steps, [
            "go-tests", "web-tests", "web-process-tests", "web-interpreter-tests", "tool-tests", "codegen-tests",
        ])
        self.assertIn("if: inputs.collect_coverage", artifact)
        self.assertIn("uses: actions/upload-artifact@v7", artifact)
        self.assertIn("name: coverage-static", artifact)
        self.assertIn("if-no-files-found: error", artifact)
        self.assertIn("overwrite: true", artifact)

    def test_contract_tests_preserve_order_and_stop_on_failure(self):
        script = step_script(self.steps, "Test CI contracts")
        expected = [
            ".github/scripts/runtime_build_contract_test.py",
            ".github/scripts/release/workflow_test.py",
            ".github/scripts/release_bump_test.py",
            ".github/scripts/driverbundle/workflow_test.py",
            "cmd/ispx/build_test.py",
        ]
        trace = r'''python3() { printf '%s:%s\n' "$PYTHONDONTWRITEBYTECODE" "$*"; [ "$*" != "${FAIL_SCRIPT:-}" ]; }
''' + script
        for failed in (None, *range(len(expected))):
            with self.subTest(failed=failed):
                result = run_shell(trace, env={"FAIL_SCRIPT": expected[failed] if failed is not None else ""})
                self.assertEqual(result.returncode, 0 if failed is None else 1, result.stderr)
                count = len(expected) if failed is None else failed + 1
                self.assertEqual(result.stdout.splitlines(), [f"1:{name}" for name in expected[:count]])

    def test_runtime_fingerprints_preserve_order_and_stop_on_failure(self):
        script = step_script(self.steps, "Check runtime fingerprints")
        expected = [f"run .github/scripts/runtime/digest.go {name} HEAD" for name in ("pack-source", "build-recipe")]
        trace = r'''go() { printf '%s\n' "$*" >&2; [ "$*" != "${FAIL_CALL:-}" ]; }
''' + script
        for failed in (None, *range(len(expected))):
            with self.subTest(failed=failed):
                result = run_shell(trace, env={"FAIL_CALL": expected[failed] if failed is not None else ""})
                self.assertEqual(result.returncode, 0 if failed is None else 1, result.stderr)
                count = len(expected) if failed is None else failed + 1
                self.assertEqual(result.stderr.splitlines(), expected[:count])
                self.assertEqual(result.stdout, "")


class CoverageWorkflowTest(unittest.TestCase):
    def test_only_runner_uploads_after_all_producers_succeed(self):
        workflow = RUNNER_WORKFLOW_PATH.read_text(encoding="utf-8")
        jobs = job_blocks(workflow)
        coverage = jobs["coverage"]
        self.assertEqual(job_needs(coverage), ("static-checks", "go-builds"))
        self.assertEqual(job_condition(coverage), "")  # Default success() is required.
        self.assertIn("coverage", job_needs(jobs["ci-gate"]))
        self.assertIn("COVERAGE_RESULT: ${{ needs.coverage.result }}", jobs["ci-gate"])
        self.assertIn("collect_coverage: true", jobs["static-checks"])
        self.assertIn("files: ${{ steps.profiles.outputs.coverage }}", coverage)
        self.assertIn("slug: goplus/spx", coverage)
        self.assertIn("disable_search: true", coverage)
        for path in RUNNER_WORKFLOW_PATH.parent.glob("*.yml"):
            with self.subTest(workflow=path.name):
                self.assertEqual(path.read_text().count("uses: codecov/codecov-action@"),
                                 1 if path == RUNNER_WORKFLOW_PATH else 0)
        downloads = coverage.split("uses: actions/download-artifact@v8")[1:]
        self.assertEqual(len(downloads), 1)
        settings = downloads[0].split("\n      - name:")[0]
        self.assertIn("pattern: coverage-*", settings)
        self.assertIn("path: coverage", settings)
        self.assertNotIn("merge-multiple: true", settings)
        static = STATIC_CHECKS_WORKFLOW_PATH.read_text()
        self.assertIn("default: false", workflow_input_block(static, "collect_coverage"))
        # Release assembly downloads all artifacts; coverage must stay out of that run.
        release = job_blocks(WORKFLOW_PATH.read_text())["static-checks"]
        self.assertNotIn("collect_coverage:", release)

    def test_collection_requires_every_source_and_keeps_nested_profiles(self):
        job = job_blocks(RUNNER_WORKFLOW_PATH.read_text())["coverage"]
        script = step_script(job, "Collect coverage profiles")
        sources = {
            "static": ["coverage.out", "cmd/ispx/coverage.out", "internal/cmd/codegen/coverage.out",
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
                    for name in ("coverage.out", "coverage/coverage-unrelated/coverage.out"):
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
                    result = run_shell(script, cwd=root, env={"GITHUB_OUTPUT": str(output)})
                    if missing:
                        self.assertNotEqual(result.returncode, 0)
                        self.assertIn(f"No coverage profiles found for {missing}", result.stderr)
                        self.assertFalse(output.exists())
                    else:
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertEqual(output.read_text(), f"coverage={','.join(sorted(profiles))}\n")


class GoBuildsWorkflowTest(unittest.TestCase):
    def test_native_coverage_uses_existing_build_matrix_and_unique_uploads(self):
        jobs = job_blocks(GO_BUILDS_WORKFLOW_PATH.read_text(encoding="utf-8"))
        self.assertEqual(list(jobs), ["go-build"])
        job = jobs["go-build"]
        self.assertIn("    runs-on: ${{ matrix.os }}", job)
        self.assertEqual(re.findall(r"- os: (\S+)\n\s+coverage-suite: (\S+)", job), [
            ("ubuntu-latest", "''"), ("macos-latest", "macos"), ("windows-latest", "windows"),
        ])
        blocks = {
            block.splitlines()[0]: block
            for block in job.split("      - name: ")[1:]
        }
        test = blocks["Test native platform behavior"]
        self.assertIn("id: native-tests", test)
        self.assertIn("COVERAGE_SUITE: ${{ matrix.coverage-suite }}", test)
        self.assertIn('run: bash .github/scripts/test_go_coverage.sh "$COVERAGE_SUITE"', test)
        self.assertIn("shell: bash", test)
        upload = blocks["Save native coverage for combined upload"]
        for block in (test, upload):
            self.assertIn("if: matrix.coverage-suite != ''", block)
        self.assertIn("uses: actions/upload-artifact@v7", upload)
        self.assertIn("name: coverage-${{ matrix.coverage-suite }}", upload)
        self.assertIn("path: ${{ steps.native-tests.outputs.coverage }}", upload)
        self.assertIn("if-no-files-found: error", upload)
        self.assertIn("overwrite: true", upload)
        self.assertLess(job.index(test), job.index(upload))
        self.assertNotRegex(job, r"coverage(?:-[\w-]+)?\.out")
        self.assertNotRegex(job, r"GO(?:OS|ARCH):")
        self.assertNotIn("continue-on-error:", job)
        self.assertIn("if: runner.os == 'Linux'", blocks["Build cmd/ispx"])
        self.assertIn("working-directory: cmd/ispx\n        run: ./build.sh --base", blocks["Build cmd/ispx"])
        self.assertIn("working-directory: cmd/ispxnative\n        run: ./build.sh", blocks["Build cmd/ispxnative"])

    def test_build_discovery_preserves_packages_and_stops_on_failure(self):
        job = job_blocks(GO_BUILDS_WORKFLOW_PATH.read_text())["go-build"]
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


class InterpreterCIWorkflowTest(unittest.TestCase):
    def test_required_checks_build_and_test_only_base_modules(self):
        static = job_blocks(STATIC_CHECKS_WORKFLOW_PATH.read_text(encoding="utf-8"))["static-checks"]
        builds = job_blocks(GO_BUILDS_WORKFLOW_PATH.read_text(encoding="utf-8"))["go-build"]
        for job in (static, builds):
            with self.subTest(job=job.splitlines()[0]):
                self.assertIn("    env:\n      GOWORK: 'off'", job)
                self.assertNotIn("cmd/ispx/ai", job)
        self.assertIn("go generate ./cmd/spx/internal/command/...", static)
        self.assertIn("go generate ./pkg/ispx/...", static)
        self.assertIn("working-directory: cmd/ispx\n        run: ./build.sh --base", builds)

    def test_web_ci_and_publication_use_base_interpreter(self):
        for path, job_name in (
            (WEB_BUILD_WORKFLOW_PATH, "build"),
            (WORKFLOW_PATH.with_name("release_web.yml"), "web-build"),
            (WEB_PACKAGE_WORKFLOW_PATH, "build"),
        ):
            with self.subTest(workflow=path.name):
                job = job_blocks(path.read_text(encoding="utf-8"))[job_name]
                # Both asset preparation and export rebuild the interpreter.
                # Select the profile in the called workflow's job, not one step.
                settings, _ = job.split("    steps:\n", 1)
                self.assertIn("    env:\n", settings)
                self.assertIn("      GOWORK: 'off'\n", settings)
                self.assertIn("      SPX_ISPX_PROFILE: base\n", settings)
                self.assertEqual(re.findall(r"(?m)^\s*SPX_ISPX_PROFILE: (.+)$", job), ["base"])


class GoTestScriptTest(unittest.TestCase):
    def setUp(self):
        workspace = tempfile.TemporaryDirectory(prefix="spx test coverage ")
        self.addCleanup(workspace.cleanup)
        self.root = Path(workspace.name).resolve() / "repository"
        self.script = self.root / ".github" / "scripts" / "test_go.sh"
        self.script.parent.mkdir(parents=True)
        self.script.write_text(GO_TEST_SCRIPT_PATH.read_text(encoding="utf-8"), encoding="utf-8")
        self.coverage_script = self.script.with_name("test_go_coverage.sh")
        self.coverage_script.write_text(GO_COVERAGE_SCRIPT_PATH.read_text(encoding="utf-8"), encoding="utf-8")
        self.output = self.root / "github-output"
        (self.root / "cmd" / "ispx").mkdir(parents=True)
        (self.root / "internal" / "cmd" / "codegen").mkdir(parents=True)
        self.log = self.root / "go-calls.jsonl"
        self.bin = Path(workspace.name) / "bin"
        self.bin.mkdir()
        write_executable(
            self.bin / "go",
            f"#!{sys.executable}\n" + r"""import json
import os
import sys
from pathlib import Path

args = sys.argv[1:]
directory = Path.cwd().relative_to(Path(os.environ["MOCK_ROOT"])).as_posix()
call = {"directory": directory, "args": args}
if os.environ.get("MOCK_CAPTURE_PLATFORM") == "1":
    call.update({key: os.environ.get(key, "") for key in ("GOOS", "GOARCH")})
log_path = Path(os.environ["MOCK_LOG"])
with log_path.open("a", encoding="utf-8") as log:
    log.write(json.dumps(call) + "\n")
if args == ["env", "GOROOT"]:
    print("/mock/go root")
elif args == ["env", "GOHOSTOS"]:
    print(os.environ["MOCK_GOHOSTOS"])
elif args == ["env", "GOOS"]:
    print(os.environ["MOCK_GOOS"])
elif args == ["list", "./..."]:
    print(os.environ["MOCK_PACKAGES"])
elif args[0] == "test":
    for argument in args[1:]:
        if argument.startswith("-coverprofile="):
            profile = argument.split("=", 1)[1]
            Path(profile).write_text(
                "mode: atomic\n" + directory + "/source.go:1.1,2.1 1 1\n",
                encoding="utf-8",
            )
else:
    sys.exit(99)

# Fail after producing partial output; call numbers start at one.
if len(log_path.read_text(encoding="utf-8").splitlines()) == int(os.environ.get("MOCK_FAIL_CALL", "0")):
    sys.exit(7)
""",
        )

    def run_script(self, packages, *arguments, script=None, **environment):
        self.log.unlink(missing_ok=True)
        return run_command(
            ["bash", str(script or self.script), *arguments],
            cwd=self.bin,
            env={
                "PATH": f"{self.bin}{os.pathsep}{os.environ['PATH']}",
                "MOCK_ROOT": str(self.root),
                "MOCK_LOG": str(self.log),
                "MOCK_PACKAGES": "\n".join(packages),
                **environment,
            },
        )

    def recorded_calls(self):
        return [json.loads(line) for line in self.log.read_text(encoding="utf-8").splitlines()]

    def test_coverage_preserves_package_selection_arguments_and_module_reports(self):
        module = "github.com/goplus/spx/v4"
        included = [
            module, f"{module}/fs", f"{module}/testing", f"{module}/tutorials",
            f"{module}/pkg/ispx/internal/pkgextra", "example.com/other/test",
        ]
        excluded = [
            "",
            f"{module}/test", f"{module}/test/project", f"{module}/tutorial",
            f"{module}/tutorial/01-Hello", f"{module}/pkg/ispx/internal/pkg",
            f"{module}/pkg/ispx/internal/pkg/github.com/goplus/spx/v4",
        ]
        arguments = ("-v", "-coverprofile=coverage.out", "-covermode=atomic", "-run", "Test With Spaces")
        completed = self.run_script([*included, *excluded], *arguments)
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(self.recorded_calls(), [
            {"directory": ".", "args": ["list", "./..."]},
            {"directory": ".", "args": ["test", *arguments, *included]},
            {"directory": "cmd/ispx", "args": ["test", *arguments, "./..."]},
        ])
        for directory in (".", "cmd/ispx"):
            with self.subTest(directory=directory):
                self.assertEqual(
                    (self.root / directory / "coverage.out").read_text(encoding="utf-8"),
                    f"mode: atomic\n{directory}/source.go:1.1,2.1 1 1\n",
                )

    def test_discovery_errors_and_empty_selection_stop_before_testing(self):
        for name, packages, environment, code in (
            ("partial discovery", ["github.com/goplus/spx/v4"], {"MOCK_FAIL_CALL": "1"}, 7),
            ("empty discovery", [], {}, 1),
            ("only excluded packages", ["github.com/goplus/spx/v4/tutorial"], {}, 1),
        ):
            with self.subTest(name=name):
                completed = self.run_script(packages, **environment)
                self.assertEqual(completed.returncode, code, completed.stderr)
                self.assertEqual(self.recorded_calls(), [{"directory": ".", "args": ["list", "./..."]}])
                if code == 1:
                    self.assertIn("No Go packages selected for testing", completed.stderr)

    def test_failed_root_tests_do_not_run_the_second_module(self):
        module = "github.com/goplus/spx/v4"
        completed = self.run_script([module], "-race", MOCK_FAIL_CALL="2")
        self.assertEqual(completed.returncode, 7, completed.stderr)
        self.assertEqual(self.recorded_calls(), [
            {"directory": ".", "args": ["list", "./..."]},
            {"directory": ".", "args": ["test", "-race", module]},
        ])

    def coverage_suite_calls(self):
        def covered(profile, *arguments, verbose=False):
            return ["test", f"-coverprofile={profile}", "-covermode=atomic", *(["-v"] if verbose else []), *arguments]

        executor = '-exec="/mock/go root/lib/wasm/go_js_wasm_exec"'
        return {
            "macos": [
                (".", ["env", "GOHOSTOS"]),
                (".", ["env", "GOOS"]),
                (".", covered("coverage-buildctl-macos.out", "-run",
                    "^(TestCommandRunnerRepairsMacOSGoToolchainForChild|TestCommandRunnerRunCommandReturnsEnvironmentError|TestConfigureCurrentMacOSGoToolchainEnv|TestCurrentBuildEnvReturnsIndependentCopy|TestSetupJDKWithHomebrewFixture|TestCompressWasmArtifactsInstallsBrotli|TestBrotliLinuxInstallRejectsMissingReleaseAndTools)$",
                    "./internal/cmd/buildctl/shared", "./internal/cmd/buildctl/tool", "./internal/cmd/buildctl/runtimecmd", verbose=True)),
            ],
            "windows": [
                (".", ["env", "GOHOSTOS"]),
                (".", ["env", "GOOS"]),
                (".", covered("coverage-buildctl-windows.out", "-run",
                    "^Test(ReplaceFileWindows|Windows.*|FetchURLToFileReplacesExistingDestination|SConsEnvironmentCommands|SetupJDKWindowsRequiresManualInstallation)$",
                    "./internal/cmd/buildctl/shared", "./internal/cmd/buildctl/tool", verbose=True)),
                (".", covered("coverage-engine-windows.out", "-run",
                    "^Test(DetectStaleEngineBuildLockProtectsLivePID|EngineBuildLockRecoversExitedPID|EngineLockWaitsForActiveOwner)$",
                    "./internal/cmd/buildctl/engine", verbose=True)),
            ],
            "web": [
                (".", ["env", "GOROOT"]),
                (".", covered("coverage-web.out", executor, "./internal/gdengine/binding/web", "./pkg/ispx", "./internal/coroutine")),
            ],
            "web-process": [
                (".", ["env", "GOROOT"]),
                (".", covered("coverage-web-process.out", executor, "-run",
                    "UnsupportedPlatform|^TestRunRejectsInvalidInput$|^TestRunCommand|^TestPrepareLinuxRuntimePackAssetsRejectsInvalidInputs$",
                    "./internal/cmd/buildctl/engine", "./internal/processsupervisor", "./x/xgolauncher")),
            ],
            "web-interpreter": [
                ("cmd/ispx", ["env", "GOROOT"]),
                ("cmd/ispx", covered("coverage-web.out", executor, "./...")),
            ],
            "tools": [
                (".", covered("coverage-driverbundle.out", "./.github/scripts/driverbundle", verbose=True)),
                *[(".", covered(f"coverage-{name}.out", f".github/scripts/runtime/{name}.go",
                    f".github/scripts/runtime/{name}_test.go", verbose=True)) for name in ("manifest", "resolution", "version")],
            ],
            "codegen": [
                ("internal/cmd/codegen", covered("coverage.out", "-coverpkg=./...", "./...")),
            ],
            "native": [
                (".", ["list", "./..."]),
                (".", ["test", "-v", "-coverprofile=coverage.out", "-covermode=atomic", "github.com/goplus/spx/v4"]),
                ("cmd/ispx", ["test", "-v", "-coverprofile=coverage.out", "-covermode=atomic", "./..."]),
            ],
        }

    def run_coverage_suite(self, suite, **environment):
        self.output.unlink(missing_ok=True)
        for profile in self.root.rglob("coverage*.out"):
            profile.unlink()
        return self.run_script(
            ["github.com/goplus/spx/v4"], suite, script=self.coverage_script,
            **{
                "GITHUB_OUTPUT": str(self.output),
                "GOOS": "",
                "GOARCH": "",
                "MOCK_CAPTURE_PLATFORM": "1",
                "MOCK_GOHOSTOS": {"macos": "darwin", "windows": "windows"}.get(suite, "linux"),
                "MOCK_GOOS": {"macos": "darwin", "windows": "windows"}.get(suite, "linux"),
                **environment,
            },
        )

    def test_coverage_suites_preserve_commands_platforms_and_report_every_profile(self):
        all_profiles = []
        for suite, calls in self.coverage_suite_calls().items():
            with self.subTest(suite=suite):
                result = self.run_coverage_suite(suite)
                self.assertEqual(result.returncode, 0, result.stderr)
                web = suite.startswith("web")
                self.assertEqual(self.recorded_calls(), [
                    {"directory": directory, "args": arguments,
                     "GOOS": "js" if web and arguments[0] == "test" else "",
                     "GOARCH": "wasm" if web and arguments[0] == "test" else ""}
                    for directory, arguments in calls
                ])
                profiles = [
                    (Path(directory) / argument.split("=", 1)[1]).as_posix()
                    for directory, arguments in calls
                    for argument in arguments if argument.startswith("-coverprofile=")
                ]
                self.assertEqual(self.output.read_text(encoding="utf-8").splitlines(), [
                    "coverage<<COVERAGE_PATHS", *profiles, "COVERAGE_PATHS",
                ])
                actual = [profile.relative_to(self.root).as_posix() for profile in self.root.rglob("coverage*.out")]
                self.assertCountEqual(actual, profiles)
                all_profiles.extend(profiles)
        self.assertEqual(len(all_profiles), len(set(all_profiles)))

    def test_failed_coverage_commands_stop_the_suite_without_publishing_outputs(self):
        for suite, calls in self.coverage_suite_calls().items():
            for index in range(1, len(calls) + 1):
                with self.subTest(suite=suite, call=index):
                    result = self.run_coverage_suite(suite, MOCK_FAIL_CALL=str(index))
                    self.assertEqual(result.returncode, 7, result.stderr)
                    self.assertEqual(
                        [(call["directory"], call["args"]) for call in self.recorded_calls()], calls[:index],
                    )
                    self.assertFalse(self.output.exists())

    def test_platform_coverage_rejects_wrong_host_and_cross_compilation(self):
        for suite, expected in (("macos", "darwin"), ("windows", "windows")):
            for variable in ("MOCK_GOHOSTOS", "MOCK_GOOS"):
                with self.subTest(suite=suite, variable=variable):
                    result = self.run_coverage_suite(suite, **{variable: "linux"})
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assertIn(f"requires a native {expected} runner", result.stderr)
                    self.assertEqual([call["args"] for call in self.recorded_calls()], [
                        ["env", "GOHOSTOS"], ["env", "GOOS"],
                    ])
                    self.assertFalse(self.output.exists())
                    self.assertEqual(list(self.root.rglob("coverage*.out")), [])

    def test_coverage_runner_rejects_unknown_suites_and_supports_local_runs(self):
        result = self.run_coverage_suite("unknown")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.log.exists())
        self.assertFalse(self.output.exists())
        result = self.run_coverage_suite("tools", GITHUB_OUTPUT="")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(self.output.exists())


class StandaloneActionTest(unittest.TestCase):
    library = WORKFLOW_PATH.parents[1] / "actions/standalone/lib/platform.sh"

    def test_binary_architecture_patterns(self):
        script = '\n'.join((
            'source "$LIBRARY"',
            'file() { printf "%s\\n" "$DESCRIPTION"; }',
            'assert_binary_arch "binary with spaces" "$PLATFORM" "$ARCH"',
        ))
        targets = (
            ("linux", "x64", "ELF 64-bit LSB executable, x86-64"),
            ("linux", "x86", "ELF 32-bit LSB executable, Intel 80386"),
            ("linux", "arm64", "ELF 64-bit LSB executable, ARM aarch64"),
            ("windows", "x64", "PE32+ executable (console) x86-64"),
            ("windows", "x86", "PE32 executable (console) Intel 80386"),
        )
        for platform, arch, expected_description in targets:
            for description in [*(item[2] for item in targets), "data"]:
                with self.subTest(platform=platform, arch=arch, description=description):
                    result = run_shell(script, env={
                        "LIBRARY": str(self.library), "PLATFORM": platform,
                        "ARCH": arch, "DESCRIPTION": description,
                    })
                    valid = description == expected_description
                    self.assertEqual(result.returncode, 0 if valid else 1, result.stderr)
                    if not valid:
                        self.assertIn(f"Binary architecture mismatch for {platform}/{arch}", result.stderr)

    def test_package_tool_precedence_and_failures(self):
        action = self.library.parents[1] / "package/action.yml"
        script = step_script(action.read_text(encoding="utf-8"), "Package standalone release")
        mocks = """
command() {
  if [ "$1" = -v ]; then
    case "$2" in
      zip) [ "$HAS_ZIP" = true ]; return ;;
      powershell.exe) [ "$HAS_POWERSHELL" = true ]; return ;;
      cygpath) return 1 ;;
    esac
  fi
  builtin command "$@"
}
zip() { echo zip; return "$TOOL_EXIT"; }
powershell.exe() { echo powershell; return "$TOOL_EXIT"; }
"""
        for zip_available, powershell_available, tool_exit in (
            (True, True, 0), (False, True, 0), (False, False, 0),
            (True, True, 7), (False, True, 7),
        ):
            with self.subTest(zip=zip_available, powershell=powershell_available, exit=tool_exit):
                with tempfile.TemporaryDirectory(prefix="standalone package ") as directory:
                    root = Path(directory)
                    library = root / ".github/actions/standalone/lib/platform.sh"
                    library.parent.mkdir(parents=True)
                    library.write_text(self.library.read_text(encoding="utf-8"), encoding="utf-8")
                    binary = root / "spx.exe"
                    binary.write_text("binary", encoding="utf-8")
                    result = run_shell(mocks + script, cwd=root, env={
                        "SPX_BIN": str(binary), "PACKAGE_FILE": "package with spaces.zip",
                        "HAS_ZIP": str(zip_available).lower(),
                        "HAS_POWERSHELL": str(powershell_available).lower(),
                        "TOOL_EXIT": str(tool_exit),
                    })
                    available = zip_available or powershell_available
                    self.assertEqual(result.returncode, tool_exit if available else 1, result.stderr)
                    self.assertEqual(result.stdout.count("[ok] Packaged standalone SPX:"), int(available and tool_exit == 0))
                    if available:
                        self.assertIn("\nzip\n" if zip_available else "\npowershell\n", result.stdout)
                    if zip_available:
                        self.assertNotIn("\npowershell\n", result.stdout)


if __name__ == "__main__":
    unittest.main()
