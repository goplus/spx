#!/usr/bin/env python3

from pathlib import Path
import sys
import tempfile
import unittest

from test_support import REPO_ROOT, SCRIPTS, run_command

VALID_INPUTS = {
    "OPERATION_INPUT": "dry-run", "CURRENT_REPOSITORY": "goplus/spx",
    "RELEASE_TAG_INPUT": "v4.0.0", "EXPECTED_RELEASE_TAG": "v4.0.0",
    "RELEASE_REPOSITORY": "goplus/spx", "PLATFORMS_INPUT": "all",
}


def run_plan(command, *, env=None, cwd=REPO_ROOT):
    return run_command([sys.executable, str(SCRIPTS / "release/plan.py"), command], cwd=cwd, env=env)


class ReleasePlanTest(unittest.TestCase):
    def test_cli_requires_a_supported_command(self):
        for command in ("", "unknown", "platforms"):
            with self.subTest(command=command):
                result = run_plan(command)
                self.assertEqual(result.returncode, 2)
                self.assertIn("usage:", result.stderr)

    def test_platform_and_lock_validation_need_no_external_tools(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            output.write_text("existing=output\n")
            result = run_plan("inputs", env={
                **VALID_INPUTS, "PATH": "", "PLATFORMS_INPUT": " WEB , Linux ",
                "GITHUB_OUTPUT": str(output),
            })
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(output.read_text(), "existing=output\nrun_web=true\nrun_macos=false\n"
                             "run_windows=false\nrun_linux=true\n")
            result = run_plan("dev-target", env={
                "PATH": "", "CURRENT_REPOSITORY": "goplus/spx",
                "CURRENT_REF": "refs/heads/dev", "RELEASE_TAG_INPUT": "",
            })
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_release_input_validation(self):
        for operation, repository, tag, ok in (
            ("dry-run", "someone/spx", "v4.0.0", True),
            ("publish-runtime", "goplus/spx", "v4.0.0", True),
            ("publish-release", "goplus/spx", "v4.0.0", True),
            ("publish-release", "someone/spx", "v4.0.0", False),
            ("publish-runtime", "someone/spx", "v4.0.0", False),
            ("dry-run", "goplus/spx", "v4.0.1", False),
            ("dry-run", "goplus/spx", "", False),
            ("unknown", "goplus/spx", "v4.0.0", False),
            ("", "goplus/spx", "v4.0.0", False),
        ):
            with self.subTest(operation=operation, repository=repository, tag=tag), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                output.write_text("existing=output\n")
                result = run_plan("inputs", env={
                    **VALID_INPUTS, "OPERATION_INPUT": operation, "CURRENT_REPOSITORY": repository,
                    "RELEASE_TAG_INPUT": tag, "GITHUB_OUTPUT": str(output),
                })
                self.assertEqual(result.returncode == 0, ok, result.stderr)
                expected = "existing=output\n"
                if ok:
                    expected += "".join(f"run_{platform}={str(operation != 'publish-runtime').lower()}\n"
                                        for platform in ("web", "macos", "windows", "linux"))
                self.assertEqual(output.read_text(), expected)

    def test_release_and_development_runtime_policies(self):
        for command in ("runtime-policy", "dev-runtime-policy"):
            for state in ("ready", "missing", "incompatible", "unknown", ""):
                with self.subTest(command=command, state=state):
                    result = run_plan(command, env={
                        "RUNTIME_STATE": state, "RUNTIME_REASON": "runtime input identity changed",
                    })
                    succeeds = state == "ready" or (state == "missing" and command == "runtime-policy")
                    self.assertEqual(result.returncode == 0, succeeds, result.stderr)
                    if state == "incompatible":
                        self.assertIn("Runtime version bump required", result.stdout)
                        self.assertIn("make bump-release", result.stderr)
                    if state == "missing" and command == "dev-runtime-policy":
                        self.assertIn("Published runtime required", result.stdout)

    def test_ancestry_summary_distinguishes_reuse_candidate_and_failures(self):
        for state, ancestry, ref_commit, succeeds, message in (
            ("ready", "", "", True, "immutable runtime reuse"),
            ("missing", "ancestor", "b" * 40, True, "eligible for publication"),
            ("missing", "premerge", "b" * 40, True, "publication is blocked"),
            ("missing", "failure", "b" * 40, False, "verification failed"),
            ("missing", "ancestor", "", False, "incomplete identity outputs"),
        ):
            with self.subTest(state=state, ancestry=ancestry, complete=bool(ref_commit)), tempfile.TemporaryDirectory() as directory:
                summary = Path(directory) / "summary"
                result = run_plan("ancestry-summary", env={
                    "RUNTIME_STATE": state, "RUNTIME_REASON": "identity matches",
                    "ANCESTRY_STATUS": ancestry, "GODOT_COMMIT": "a" * 40,
                    "GODOT_REPOSITORY": "https://github.com/goplus/godot.git",
                    "GODOT_REF": "canonical", "GODOT_REF_COMMIT": ref_commit,
                    "OPERATION_INPUT": "dry-run", "GITHUB_STEP_SUMMARY": str(summary),
                })
                self.assertEqual(result.returncode == 0, succeeds, result.stderr)
                self.assertIn(message, summary.read_text())

    def test_release_platform_selection_preserves_output_order_and_validation(self):
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
                result = run_plan("inputs", env={
                    **VALID_INPUTS, "OPERATION_INPUT": operation, "PLATFORMS_INPUT": platforms,
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

    def test_development_requires_canonical_repository_branch_and_no_tag(self):
        cases = (
            ("goplus/spx", "refs/heads/dev", "", True),
            ("someone/spx", "refs/heads/dev", "", False),
            ("goplus/spx", "refs/heads/release/v4.0.0", "", False),
            ("goplus/spx", "refs/heads/dev", "v4.0.0", False),
        )
        for repository, ref, release_tag, succeeds in cases:
            with self.subTest(repository=repository, ref=ref, release_tag=release_tag):
                completed = run_plan(
                    "dev-target",
                    cwd=REPO_ROOT,
                    env={
                        "CURRENT_REF": ref,
                        "CURRENT_REPOSITORY": repository,
                        "RELEASE_TAG_INPUT": release_tag,
                    },
                )
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)


if __name__ == "__main__":
    unittest.main()
