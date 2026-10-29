#!/usr/bin/env python3

import contextlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

import publish


def run_publish(command, env, external):
    stdout, stderr = io.StringIO(), io.StringIO()
    with mock.patch.dict(os.environ, env, clear=True), mock.patch.object(sys, "argv", ["publish.py", command]), \
            mock.patch.object(publish, "run", side_effect=external), \
            contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        result = publish.main()
    return result, stdout.getvalue(), stderr.getvalue()


class ReleasePublishTest(unittest.TestCase):
    def test_immutable_target_preflight_preserves_runtime_and_product_policy(self):
        for operation, state, commit, marker, tag_exists, succeeds in (
            ("publish-runtime", "public", "b" * 40, True, False, True),
            ("publish-release", "public", "b" * 40, True, False, False),
            ("publish-release", "public", "a" * 40, True, False, True),
            ("publish-runtime", "draft", "b" * 40, True, False, False),
            ("publish-runtime", "draft", "a" * 40, False, False, True),
            ("publish-runtime", "public", "a" * 40, False, False, False),
            ("publish-runtime", "absent", "a" * 40, False, False, True),
            ("publish-runtime", "absent", "a" * 40, False, True, False),
        ):
            with self.subTest(operation=operation, state=state, commit=commit, marker=marker, tag=tag_exists):
                metadata = {"isDraft": state == "draft", "assets": [
                    {"name": name} for name in ("runtime-manifest.json", "SHA256SUMS")
                ] if marker else []}

                def external(*args, **kwargs):
                    if args[:2] == ("gh", "api"):
                        text = json.dumps({"sha": commit}) if "/commits/" in args[2] else ""
                        return subprocess.CompletedProcess(args, 0, text)
                    if args[:3] == ("gh", "release", "view"):
                        return subprocess.CompletedProcess(args, 1 if state == "absent" else 0, json.dumps(metadata))
                    if args[:2] == ("git", "ls-remote"):
                        return subprocess.CompletedProcess(args, 0 if tag_exists else 2, "")
                    self.fail(f"Unexpected command: {args}")

                result, _, stderr = run_publish("validate-targets", {
                    "REPOSITORY": "fixture/spx", "OPERATION": operation,
                    "RUNTIME_TAG": "runtime-v4.0.0", "RELEASE_TAG": "v4.0.0",
                    "RUNTIME_MANIFEST": "runtime-manifest.json", "GITHUB_SHA": "a" * 40,
                }, external)
                self.assertEqual(result == 0, succeeds, stderr)

    def test_remote_tag_query_errors_fail_with_diagnostics_at_cli_boundary(self):
        with tempfile.TemporaryDirectory() as directory:
            fake_bin = Path(directory)
            fixture = f"#!{sys.executable}\n" + '''
import os
from pathlib import Path
import sys

tool = Path(sys.argv[0]).name
if tool == "gh":
    if sys.argv[1:3] == ["api", "repos/fixture/spx"]:
        sys.exit(0)
    if sys.argv[1:3] == ["release", "view"]:
        sys.exit(1)
elif tool == "git" and sys.argv[1] == "ls-remote":
    if sys.argv[-1] == "refs/tags/" + os.environ["FAILED_TAG"]:
        print(os.environ["QUERY_DIAGNOSTIC"], file=sys.stderr)
        sys.exit(int(os.environ["QUERY_EXIT"]))
    sys.exit(2)
raise SystemExit("Unexpected external command: " + repr(sys.argv))
'''
            for tool in ("gh", "git"):
                executable = fake_bin / tool
                executable.write_text(fixture)
                executable.chmod(0o755)
            for tag in ("runtime-v4.0.0", "v4.0.0"):
                for exit_code, diagnostic in ((128, "fatal: unable to access repository"), (1, "")):
                    with self.subTest(tag=tag, exit_code=exit_code):
                        result = subprocess.run(
                            [sys.executable, publish.__file__, "validate-targets"], capture_output=True, text=True,
                            env={**os.environ, "PATH": str(fake_bin), "REPOSITORY": "fixture/spx",
                                 "OPERATION": "publish-release", "RUNTIME_TAG": "runtime-v4.0.0",
                                 "RELEASE_TAG": "v4.0.0", "RUNTIME_MANIFEST": "runtime-manifest.json",
                                 "GITHUB_SHA": "a" * 40, "FAILED_TAG": tag,
                                 "QUERY_EXIT": str(exit_code), "QUERY_DIAGNOSTIC": diagnostic},
                        )
                        self.assertEqual(result.returncode, 1, result.stdout)
                        self.assertIn(f"tag {tag}: git ls-remote exited {exit_code}", result.stderr)
                        if diagnostic:
                            self.assertIn(diagnostic, result.stderr)

    def test_publish_steps_reuse_metadata_and_stop_before_upload_on_failure(self):
        for runtime in (True, False):
            scenarios = ["absent", "draft", "public", "extra-assets", "download-failure",
                         "checksum-failure", "different-checksums", "create-failure",
                         "delete-asset-failure", "upload-failure"]
            if runtime:
                scenarios.extend(("manifest-failure", "edit-failure"))
            for scenario in scenarios:
                with self.subTest(runtime=runtime, scenario=scenario), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    for folder in ("dist", "remote"):
                        (root / folder).mkdir()
                        for name in ("asset.zip", "manifest.json", "SHA256SUMS"):
                            (root / folder / name).write_text("same fixture contents\n")
                    if scenario == "extra-assets":
                        (root / "remote" / "extra.zip").write_text("unexpected\n")
                    if scenario == "different-checksums":
                        (root / "remote" / "SHA256SUMS").write_text("different checksums\n")
                    state = scenario if scenario in ("absent", "draft") else "public"
                    if scenario in ("create-failure", "upload-failure", "edit-failure"):
                        state = "absent"
                    if scenario == "delete-asset-failure":
                        state = "draft"
                    metadata = {"isDraft": state == "draft", "assets": [
                        {"name": path.name} for path in sorted((root / "remote").iterdir())
                    ]}
                    trace = []

                    def external(*args, **kwargs):
                        operation = args[2] if args[0] == "gh" else {"sha256sum": "checksum", "go": "manifest"}[args[0]]
                        trace.append(operation)
                        if scenario == operation + "-failure":
                            raise subprocess.CalledProcessError(7, args)
                        if operation == "view":
                            self.assertEqual(args[-2:], ("--json", "isDraft,assets"))
                            return subprocess.CompletedProcess(args, 1 if state == "absent" else 0, json.dumps(metadata))
                        if operation == "download":
                            shutil.copytree(root / "remote", root / "published", dirs_exist_ok=True)
                        if operation == "manifest":
                            self.assertIn("--verify-manifest", args)
                            self.assertIn("./.github/cmd/runtime-manifest", args)
                        return subprocess.CompletedProcess(args, 0, "")

                    previous = Path.cwd()
                    os.chdir(root)
                    try:
                        result, _, stderr = run_publish("runtime" if runtime else "spx", {
                            "REPOSITORY": "fixture/spx", "RELEASE_TAG": "v4.0.0", "GITHUB_SHA": "a" * 40,
                            "RUNTIME_STATE": "missing", "RUNTIME_VERSION": "4.0.0", "RUNTIME_MANIFEST": "manifest.json",
                        }, external)
                    finally:
                        os.chdir(previous)
                    succeeds = scenario in ("absent", "draft", "public") or (scenario == "different-checksums" and runtime)
                    self.assertEqual(result == 0, succeeds, stderr)
                    self.assertEqual(trace.count("view"), 1)
                    writes = [event for event in trace if event in ("create", "delete-asset", "upload", "edit")]
                    expected = []
                    if state in ("absent", "draft"):
                        expected = ["create"] if state == "absent" else ["delete-asset"] * len(metadata["assets"])
                        expected += ["upload"] + (["edit"] if runtime else [])
                        if scenario.endswith("-failure"):
                            expected = expected[:expected.index(scenario.removesuffix("-failure")) + 1]
                    self.assertEqual(writes, expected)
                    if scenario == "public":
                        self.assertIn("checksum", trace)
                        self.assertEqual("manifest" in trace, runtime)

    def test_invalid_command_fails_at_cli_boundary(self):
        result = subprocess.run([sys.executable, publish.__file__, "unknown"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn("usage:", result.stderr)


if __name__ == "__main__":
    unittest.main()
