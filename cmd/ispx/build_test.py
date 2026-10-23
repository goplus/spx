#!/usr/bin/env python3

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class InterpreterBuildTest(unittest.TestCase):
    def setUp(self):
        workspace = tempfile.TemporaryDirectory(prefix="spx interpreter build ")
        self.addCleanup(workspace.cleanup)
        self.root = Path(workspace.name).resolve()
        self.module = self.root / "cmd" / "ispx"
        (self.module / "ai").mkdir(parents=True)
        self.script = self.module / "build.sh"
        self.script.write_text(Path(__file__).with_name("build.sh").read_text(encoding="utf-8"), encoding="utf-8")
        self.output = self.module / "ispx.wasm"
        self.log = self.root / "go-calls.jsonl"
        self.other_directory = self.root / "other working directory"
        self.other_directory.mkdir()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        go = self.bin / "go"
        go.write_text(f"#!{sys.executable}\n" + r'''import json
import os
from pathlib import Path
import sys

args = sys.argv[1:]
call = {
    "directory": str(Path.cwd()),
    "arguments": args,
    "environment": {key: os.environ.get(key) for key in ("GOOS", "GOARCH", "GOWORK", "GOTOOLCHAIN", "CGO_ENABLED")},
}
with Path(os.environ["MOCK_GO_LOG"]).open("a", encoding="utf-8") as log:
    log.write(json.dumps(call) + "\n")
code = int(os.environ["MOCK_GO_EXIT"])
if code:
    sys.exit(code)
Path(args[args.index("-o") + 1]).write_text(Path.cwd().name, encoding="utf-8")
''', encoding="utf-8")
        go.chmod(0o755)

    def build(self, *arguments, profile=None, go_exit=0):
        self.log.unlink(missing_ok=True)
        environment = dict(os.environ)
        environment.pop("SPX_ISPX_PROFILE", None)
        environment.update({
            "PATH": f"{self.bin}{os.pathsep}{environment['PATH']}",
            "MOCK_GO_LOG": str(self.log),
            "MOCK_GO_EXIT": str(go_exit),
            "GOOS": "linux", "GOARCH": "arm64",
            "GOWORK": "off", "GOTOOLCHAIN": "go1.26.2", "CGO_ENABLED": "0",
        })
        if profile is not None:
            environment["SPX_ISPX_PROFILE"] = profile
        return subprocess.run(
            ["bash", str(self.script), *arguments], cwd=self.other_directory,
            env=environment, capture_output=True, text=True,
        )

    def test_profiles_build_from_any_directory_to_the_same_wasm_file(self):
        for profile, arguments, module in (
            (None, (), "ai"), ("", (), "ai"),
            ("base", (), ""), ("ai", (), "ai"),
            (None, ("--base",), ""), ("base", ("--ai",), "ai"),
            ("ai", ("--base",), ""), ("unknown", ("--base",), ""),
            ("unknown", ("--ai",), "ai"),
        ):
            with self.subTest(profile=profile, arguments=arguments):
                result = self.build(*arguments, profile=profile)
                self.assertEqual(result.returncode, 0, result.stderr)
                calls = self.log.read_text(encoding="utf-8").splitlines()
                self.assertEqual(len(calls), 1)
                call = json.loads(calls[0])
                self.assertEqual(call["directory"], str(self.module / module))
                args = call["arguments"]
                self.assertEqual(args[0], "build")
                self.assertEqual(args[-1], ".")
                self.assertEqual(args[args.index("-o") + 1], str(self.output))
                self.assertEqual(call["environment"], {
                    "GOOS": "js", "GOARCH": "wasm",
                    "GOWORK": "off", "GOTOOLCHAIN": "go1.26.2", "CGO_ENABLED": "0",
                })
                self.assertEqual(self.output.read_text(encoding="utf-8"), module or "ispx")
                self.assertEqual(list(self.root.rglob("*.wasm")), [self.output])

    def test_invalid_profile_or_arguments_do_not_invoke_go(self):
        for profile, arguments in (
            ("unknown", ()), (None, ("--unknown",)), (None, ("",)),
            (None, ("--base", "--ai")), (None, ("--help", "--base")),
        ):
            with self.subTest(profile=profile, arguments=arguments):
                result = self.build(*arguments, profile=profile)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertTrue(result.stderr)
                self.assertFalse(self.log.exists())
                self.assertFalse(self.output.exists())

    def test_help_does_not_require_a_valid_profile_or_ai_module(self):
        (self.module / "ai").rmdir()
        for argument in ("-h", "--help"):
            with self.subTest(argument=argument):
                result = self.build(argument, profile="unknown")
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("Usage:", result.stdout)
                self.assertFalse(self.log.exists())

    def test_base_build_does_not_require_ai_and_default_does_not_fall_back(self):
        (self.module / "ai").rmdir()
        for arguments, profile in ((("--base",), None), ((), "base")):
            with self.subTest(arguments=arguments, profile=profile):
                result = self.build(*arguments, profile=profile)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(self.output.read_text(encoding="utf-8"), "ispx")
        result = self.build()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.log.exists())
        self.assertEqual(self.output.read_text(encoding="utf-8"), "ispx")

    def test_go_build_failure_preserves_the_exit_code(self):
        for arguments in ((), ("--base",)):
            with self.subTest(arguments=arguments):
                result = self.build(*arguments, go_exit=7)
                self.assertEqual(result.returncode, 7, result.stderr)
                self.assertEqual(len(self.log.read_text(encoding="utf-8").splitlines()), 1)
                self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
