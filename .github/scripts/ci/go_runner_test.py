#!/usr/bin/env python3

import json
import os
from pathlib import Path
import sys
import tempfile
import unittest

from test_support import SCRIPTS, run_command, write_executable

GO_TEST_SCRIPT_PATH = SCRIPTS / "ci/test_go.sh"
GO_COVERAGE_SCRIPT_PATH = SCRIPTS / "ci/test_go_coverage.sh"


class GoTestScriptTest(unittest.TestCase):
    def setUp(self):
        workspace = tempfile.TemporaryDirectory(prefix="spx test coverage ")
        self.addCleanup(workspace.cleanup)
        self.root = Path(workspace.name).resolve() / "repository"
        self.script = self.root / ".github" / "scripts" / "ci" / "test_go.sh"
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

    coverage_suites = ("macos", "windows", "web", "web-process", "web-interpreter", "tools", "codegen", "native")

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

    def test_coverage_suites_select_targets_and_report_every_profile(self):
        all_profiles = []
        for suite in self.coverage_suites:
            with self.subTest(suite=suite):
                result = self.run_coverage_suite(suite)
                self.assertEqual(result.returncode, 0, result.stderr)
                calls = self.recorded_calls()
                tests = [call for call in calls if call["args"][0] == "test"]
                self.assertTrue(tests)
                profiles = []
                for call in tests:
                    args = call["args"]
                    self.assertIn("-covermode=atomic", args)
                    profile, = [arg.split("=", 1)[1] for arg in args if arg.startswith("-coverprofile=")]
                    profiles.append((Path(call["directory"]) / profile).as_posix())
                    self.assertEqual((call["GOOS"], call["GOARCH"]), ("js", "wasm") if suite.startswith("web") else ("", ""))
                    if suite.startswith("web"):
                        self.assertTrue(any(arg.startswith("-exec=") and "go_js_wasm_exec" in arg for arg in args))
                    if suite in ("macos", "windows"):
                        self.assertIn("-run", args)
                if suite == "tools":
                    targets = {arg for call in tests for arg in call["args"]}
                    self.assertEqual(len(tests), 1)
                    self.assertIn("./.github/cmd/...", targets)
                    self.assertFalse(any(target.endswith(".go") for target in targets))
                if suite == "windows":
                    cache_test, = [call for call in tests if "./internal/runtimebundle" in call["args"]]
                    args = cache_test["args"]
                    for name in ("TestCacheDefaultPermissionsProtectNestedFilesAndRepair",
                                 "TestCacheUsesConfiguredWindowsPermissions",
                                 "TestWindowsPrivateDACLIsAppliedAtCreation",
                                 "TestAcquireCreatesProtectedWindowsCache",
                                 "TestVerifyPrivateSecurityDescriptorUsesBinarySIDs",
                                 "TestVerifyPrivateSecurityDescriptorAcceptsExpectedForms",
                                 "TestVerifyPrivateSecurityDescriptorRejectsUnexpectedACL",
                                 "TestVerifyPrivateSecurityDescriptorRejectsMissingDACL"):
                        self.assertRegex(name, args[args.index("-run") + 1])
                if suite in ("web-interpreter", "codegen"):
                    expected = "cmd/ispx" if suite == "web-interpreter" else "internal/cmd/codegen"
                    self.assertEqual({call["directory"] for call in tests}, {expected})
                self.assertEqual(self.output.read_text().splitlines(), ["coverage<<COVERAGE_PATHS", *profiles, "COVERAGE_PATHS"])
                actual = [profile.relative_to(self.root).as_posix() for profile in self.root.rglob("coverage*.out")]
                self.assertCountEqual(actual, profiles)
                all_profiles.extend(profiles)
        self.assertEqual(len(all_profiles), len(set(all_profiles)))

    def test_failed_coverage_commands_stop_the_suite_without_publishing_outputs(self):
        for suite in self.coverage_suites:
            baseline = self.run_coverage_suite(suite)
            self.assertEqual(baseline.returncode, 0, baseline.stderr)
            calls = self.recorded_calls()
            for index in range(1, len(calls) + 1):
                with self.subTest(suite=suite, call=index):
                    result = self.run_coverage_suite(suite, MOCK_FAIL_CALL=str(index))
                    self.assertEqual(result.returncode, 7, result.stderr)
                    self.assertEqual(self.recorded_calls(), calls[:index])
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



if __name__ == "__main__":
    unittest.main()
