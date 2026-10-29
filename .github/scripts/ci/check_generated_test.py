#!/usr/bin/env python3
"""Exercise generation ordering, drift policy, and local-edit protection."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("check_generated.sh").resolve()
SNAPSHOT = "pkg/ispx/internal/pkg/github.com/goplus/spx/v4/export.types"


class GeneratedFilesTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="spx generated check ")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.script = self.root / ".github/scripts/ci/check_generated.sh"
        self.script.parent.mkdir(parents=True)
        self.script.write_text(SCRIPT.read_text())
        self.snapshot = self.root / SNAPSHOT
        self.snapshot.parent.mkdir(parents=True)
        self.snapshot.write_text("committed types\n")
        self.generated = self.root / "internal/gdengine/manager.gen.go"
        self.generated.parent.mkdir(parents=True)
        self.generated.write_text("package gdengine\n")
        (self.root / "internal/cmd/codegen").mkdir(parents=True)
        (self.root / ".gitignore").write_text("*.out\n")
        self.git("init", "-q")
        self.git("add", ".")
        self.git("-c", "user.name=CI", "-c", "user.email=ci@example.com", "-c", "commit.gpgsign=false",
                 "commit", "--no-verify", "-qm", "fixture")
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.trace = self.root / "go-calls.jsonl"
        go = self.bin / "go"
        go.write_text(f"#!{sys.executable}\n" + r'''import json, os, sys
from pathlib import Path
root = Path(os.environ["FIXTURE_ROOT"])
trace = root / "go-calls.jsonl"
with trace.open("a") as output:
    output.write(json.dumps({"args": sys.argv[1:], "cwd": str(Path.cwd().relative_to(root)),
                             "module": os.environ.get("SPX_MODULE_SRC")}) + "\n")
index = len(trace.read_text().splitlines())
if index == 3:
    snapshot = root / "pkg/ispx/internal/pkg/github.com/goplus/spx/v4/export.types"
    snapshot.write_text("generated types\n")
    generated = root / "internal/gdengine/manager.gen.go"
    change = os.environ.get("CHANGE", "none")
    if change == "modified": generated.write_text("package changed\n")
    elif change == "deleted": generated.unlink()
    elif change == "untracked": generated.with_name("new.gen.go").write_text("package gdengine\n")
    elif change == "ignored": generated.with_name("coverage.out").write_text("coverage\n")
    elif change == "unrelated": (root / "local-notes.txt").write_text("unrelated work\n")
if index == int(os.environ.get("FAIL_CALL", "0")):
    sys.exit(17)
''')
        go.chmod(0o755)

    def git(self, *args):
        return subprocess.run(["git", *args], cwd=self.root, check=True, capture_output=True, text=True)

    def run_check(self, *args, **env):
        return subprocess.run(["bash", str(self.script), *args], cwd=self.bin, capture_output=True, text=True,
                              env={**os.environ, "PATH": f"{self.bin}{os.pathsep}{os.environ['PATH']}",
                                   "FIXTURE_ROOT": str(self.root), "SPX_MODULE_SRC": "external-module", **env})

    def calls(self):
        return [json.loads(line) for line in self.trace.read_text().splitlines()] if self.trace.exists() else []

    def test_regular_ci_generates_before_restoring_only_the_type_snapshot(self):
        result = self.run_check("--check-export-types", "false")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertEqual(self.snapshot.read_text(), "committed types\n")
        self.assertEqual(self.calls(), [
            {"args": ["run", "."], "cwd": "internal/cmd/codegen", "module": str(self.root / "godot_modules/spx")},
            {"args": ["generate", "./cmd/spx/internal/command/..."], "cwd": ".", "module": "external-module"},
            {"args": ["generate", "./pkg/ispx/..."], "cwd": ".", "module": "external-module"},
        ])

    def test_strict_mode_is_default_and_rejects_type_snapshot_drift(self):
        for args in ((), ("--check-export-types", "true")):
            with self.subTest(args=args):
                self.git("restore", "--", SNAPSHOT)
                self.trace.unlink(missing_ok=True)
                result = self.run_check(*args)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.snapshot.read_text(), "generated types\n")
                self.assertEqual(len(self.calls()), 3)

    def test_other_generated_changes_fail_and_unrelated_files_do_not(self):
        for change, expected in (("modified", 1), ("deleted", 1), ("untracked", 1),
                                 ("ignored", 0), ("unrelated", 0)):
            with self.subTest(change=change):
                self.git("restore", "--", "internal/gdengine")
                self.generated.with_name("new.gen.go").unlink(missing_ok=True)
                self.trace.unlink(missing_ok=True)
                result = self.run_check("--check-export-types", "false", CHANGE=change)
                self.assertEqual(result.returncode, expected, result.stderr + result.stdout)

    def test_generator_failure_stops_following_phases_and_does_not_restore(self):
        for index in (1, 2, 3):
            with self.subTest(index=index):
                self.git("restore", "--", SNAPSHOT)
                self.trace.unlink(missing_ok=True)
                result = self.run_check("--check-export-types", "false", FAIL_CALL=str(index))
                self.assertEqual(result.returncode, 17, result.stderr)
                self.assertEqual(len(self.calls()), index)
                self.assertEqual(self.snapshot.read_text(), "generated types\n" if index == 3 else "committed types\n")

    def test_local_or_staged_snapshot_edits_are_preserved_before_generation(self):
        for staged in (False, True):
            with self.subTest(staged=staged):
                self.snapshot.write_text("local changes\n")
                if staged:
                    self.git("add", SNAPSHOT)
                result = self.run_check("--check-export-types", "false")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Commit or stash", result.stderr)
                self.assertEqual(self.snapshot.read_text(), "local changes\n")
                self.assertEqual(self.calls(), [])

    def test_invalid_policy_fails_before_generation(self):
        for args in (("--check-export-types", "unknown"), ("--unknown",), ("true",)):
            with self.subTest(args=args):
                result = self.run_check(*args)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(self.calls(), [])


if __name__ == "__main__":
    unittest.main()
