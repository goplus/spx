#!/usr/bin/env python3

import json
import os
import re
import subprocess
import tempfile
import unittest
from pathlib import Path


WORKFLOW_PATH = Path(__file__).resolve().parents[1] / "workflows" / "release.yml"
WEB_PACKAGE_WORKFLOW_PATH = (
    Path(__file__).resolve().parents[1] / "workflows" / "publish_web_package.yml"
)


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
        start = lines.index(f"      - name: {name}")
        run_line = lines.index("        run: |", start + 1)
    except ValueError as exc:
        raise AssertionError(f"missing literal run script for step {name!r}") from exc

    body = []
    for line in lines[run_line + 1 :]:
        if line and not line.startswith("          "):
            break
        body.append(line[10:] if line else line)
    return "\n".join(body)


def workflow_dispatch_input_block(text, name):
    lines = text.splitlines()
    try:
        start = lines.index(f"      {name}:")
    except ValueError as exc:
        raise AssertionError(f"missing workflow_dispatch input {name!r}") from exc

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
        cls.web_package_workflow = WEB_PACKAGE_WORKFLOW_PATH.read_text(encoding="utf-8")
        cls.web_package_jobs = job_blocks(cls.web_package_workflow)

    def test_publish_dev_input_contract(self):
        release_tag = workflow_dispatch_input_block(self.workflow, "release_tag")
        self.assertIn("        required: false", release_tag)
        self.assertIn("        default: ''", release_tag)

        operation = workflow_dispatch_input_block(self.workflow, "operation")
        self.assertIn("          - publish-dev-npm", operation)

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

    def test_publish_dev_uses_exact_sha_and_oidc(self):
        block = self.jobs["publish-dev-web-package"]
        self.assertIn("    uses: ./.github/workflows/publish_web_package.yml", block)
        self.assertIn("      target: ${{ github.sha }}", block)
        self.assertIn("      release_version: ''", block)
        self.assertIn("      release_artifact: ''", block)
        self.assertIn("      contents: read", block)
        self.assertIn("      id-token: write", block)

        guard = step_script(
            self.jobs["dev-npm-guard"], "Require the canonical dev branch"
        )
        cases = (
            ("goplus/spx", "refs/heads/dev", "", True),
            ("someone/spx", "refs/heads/dev", "", False),
            ("goplus/spx", "refs/heads/release/v3.2.0", "", False),
            ("goplus/spx", "refs/heads/dev", "v3.2.0", False),
        )
        for repository, ref, release_tag, succeeds in cases:
            with self.subTest(repository=repository, ref=ref, release_tag=release_tag):
                completed = subprocess.run(
                    ["bash", "-euo", "pipefail", "-c", guard],
                    cwd=WORKFLOW_PATH.parents[2],
                    env={
                        **os.environ,
                        "CURRENT_REF": ref,
                        "CURRENT_REPOSITORY": repository,
                        "RELEASE_TAG_INPUT": release_tag,
                    },
                    capture_output=True,
                    text=True,
                    check=False,
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
            ("a" * 40, "3.2.0", "web-release", True),
            ("a" * 40, "3.2.0", "", False),
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
                completed = subprocess.run(
                    ["bash", "-euo", "pipefail", "-c", validate_target],
                    env={
                        **os.environ,
                        "RELEASE_ARTIFACT": release_artifact,
                        "RELEASE_VERSION": release_version,
                        "TARGET": target,
                    },
                    capture_output=True,
                    text=True,
                    check=False,
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
                ("git", "tag", "v3.2.0"),
            ):
                subprocess.run(command, cwd=repository, check=True)
            target = subprocess.run(
                ("git", "rev-parse", "HEAD"),
                cwd=repository,
                capture_output=True,
                text=True,
                check=True,
            ).stdout.strip()

            tagged = subprocess.run(
                ["bash", "-euo", "pipefail", "-c", verify_source],
                cwd=repository,
                env={
                    **os.environ,
                    "IS_DEVELOPMENT": "true",
                    "TARGET": target,
                },
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(tagged.returncode, 0, tagged.stdout)

            wrong_sha = subprocess.run(
                ["bash", "-euo", "pipefail", "-c", verify_source],
                cwd=repository,
                env={
                    **os.environ,
                    "IS_DEVELOPMENT": "false",
                    "TARGET": "b" * 40,
                },
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(wrong_sha.returncode, 0, wrong_sha.stdout)
            self.assertIn("instead of publication target", wrong_sha.stderr)

            subprocess.run(
                ("git", "tag", "--delete", "v3.2.0"),
                cwd=repository,
                capture_output=True,
                check=True,
            )
            untagged = subprocess.run(
                ["bash", "-euo", "pipefail", "-c", verify_source],
                cwd=repository,
                env={
                    **os.environ,
                    "IS_DEVELOPMENT": "true",
                    "TARGET": target,
                },
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(untagged.returncode, 0, untagged.stderr)

        publish = step_script(block, "Publish web package to npm")
        with tempfile.TemporaryDirectory() as workspace:
            workspace_path = Path(workspace)
            scripts_path = workspace_path / ".github" / "scripts"
            scripts_path.mkdir(parents=True)
            prepare = scripts_path / "prepare_spx_web_package.sh"
            prepare.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
            prepare.chmod(0o755)

            fake_bin = workspace_path / "fake-bin"
            fake_bin.mkdir()
            fake_npm = fake_bin / "npm"
            fake_npm.write_text(
                """#!/bin/sh
set -eu
case "$1:$3" in
  pack:*)
    printf '%s\\n' '[{"filename":"spx.tgz","integrity":"sha512-local"}]'
    ;;
  view:version)
    printf '%s\\n' "$MOCK_VERSION"
    ;;
  view:dist.integrity)
    printf '%s\\n' 'sha512-local'
    ;;
  view:dist-tags)
    printf '{"dev":"%s"}\\n' "$MOCK_DIST_TAG"
    ;;
  *)
    exit 99
    ;;
esac
""",
                encoding="utf-8",
            )
            fake_npm.chmod(0o755)

            version = "3.2.1-0.20260814000000-aaaaaaaaaaaa"
            for remote_dist_tag, succeeds in ((version, True), ("older", False)):
                with self.subTest(remote_dist_tag=remote_dist_tag):
                    completed = subprocess.run(
                        ["bash", "-euo", "pipefail", "-c", publish],
                        cwd=workspace,
                        env={
                            **os.environ,
                            "IS_RELEASE": "false",
                            "MOCK_DIST_TAG": remote_dist_tag,
                            "MOCK_VERSION": version,
                            "PATH": f"{fake_bin}:{os.environ['PATH']}",
                            "SPX_VERSION": f"v{version}",
                        },
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    self.assertEqual(
                        completed.returncode == 0,
                        succeeds,
                        completed.stderr,
                    )

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
                completed = subprocess.run(
                    ["bash", "-euo", "pipefail", "-c", script],
                    env={
                        **os.environ,
                        "OPERATION": operation,
                        "NEEDS_JSON": json.dumps(needs_json),
                    },
                    capture_output=True,
                    text=True,
                    check=False,
                )
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)


if __name__ == "__main__":
    unittest.main()
