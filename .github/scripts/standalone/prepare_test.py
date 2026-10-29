#!/usr/bin/env python3
"""Check native artifact source selection and executable discovery."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[3]
PREPARE = ROOT / ".github/scripts/standalone/prepare.sh"


class StandalonePreparationTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="spx native preparation ")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.bin = self.root / "tools"
        self.bin.mkdir()
        self.gopath = self.root / "go path"
        (self.gopath / "bin").mkdir(parents=True)
        self.output = self.root / "output"
        self.trace = self.root / "trace"
        self.tool("go", '''#!/bin/bash
case "$*" in
  'env GOEXE') printf '%s\\n' "$TEST_GOEXE" ;;
  'env GOPATH') printf '%s\\n' "$TEST_GOPATH" ;;
  *) exit 90 ;;
esac
''')
        self.tool("buildctl", '''#!/usr/bin/env python3
import json, os, sys
with open(os.environ["TRACE"], "w") as trace:
    json.dump({"args": sys.argv[1:], "gowork": os.environ["GOWORK"]}, trace)
sys.exit(int(os.environ.get("BUILDCTL_EXIT", "0")))
''')
        self.env = {
            **os.environ,
            "PATH": f"{self.bin}{os.pathsep}{os.environ['PATH']}",
            "GITHUB_OUTPUT": str(self.output), "TRACE": str(self.trace),
            "BUILDCTL": str(self.bin / "buildctl"), "GOWORK": "outside.work",
            "ENGINE_ASSET_DIR": "", "TEST_GOPATH": str(self.gopath), "TEST_GOEXE": "",
        }

    def tool(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def prepare(self, **environment):
        return subprocess.run(["bash", str(PREPARE)], cwd=self.root,
                              env={**self.env, **environment}, text=True, capture_output=True, timeout=20)

    def test_uses_published_runtime_only_without_current_run_assets(self):
        for asset_dir in ("", str(self.root / "engine assets")):
            with self.subTest(asset_dir=asset_dir):
                (self.gopath / "bin/spx").write_text("binary")
                self.output.write_text("")
                result = self.prepare(ENGINE_ASSET_DIR=asset_dir, TEST_GOPATH=f"{self.gopath}:/another/gopath")
                self.assertEqual(result.returncode, 0, result.stderr)
                args = ["setup", "host"] + (["--asset-dir", asset_dir] if asset_dir else ["--published-runtime"])
                self.assertEqual(json.loads(self.trace.read_text()), {"args": args, "gowork": "off"})
                self.assertEqual(self.output.read_text(), f"spx-bin={self.gopath}/bin/spx\n")

    def test_windows_uses_first_gopath_and_executable_suffix(self):
        (self.gopath / "bin/spx.exe").write_text("binary")
        result = self.prepare(TEST_GOEXE=".exe", TEST_GOPATH=f"{self.gopath};C:\\other")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.output.read_text(), f"spx-bin={self.gopath}/bin/spx.exe\n")

    def test_failed_setup_or_missing_binary_produces_no_output(self):
        for status in ("17", "0"):
            with self.subTest(status=status):
                result = self.prepare(BUILDCTL_EXIT=status)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
