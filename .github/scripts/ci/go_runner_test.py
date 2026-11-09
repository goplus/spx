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
if len(args) == 2 and args[0] == "env":
    print(os.environ["MOCK_" + args[1]])
elif args == ["list", "./..."]:
    print(os.environ["MOCK_NESTED_PACKAGES" if directory == "cmd/ispx" else "MOCK_PACKAGES"])
elif args[0] == "test":
    for argument in args[1:]:
        if argument.startswith("-coverprofile="):
            profile = argument.split("=", 1)[1]
            Path(profile).write_text(
                "mode: atomic\n" + directory + "/source.go:1.1,2.1 1 1\n",
                encoding="utf-8",
            )
elif args[0] == "run" and args[1:2] == ["./.github/cmd/compact-coverage"]:
    if len(args) < 3 or not all(Path(path).is_file() for path in args[2:]):
        sys.exit(98)
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
                "MOCK_GOROOT": "/mock/go root",
                "MOCK_PACKAGES": "\n".join(packages),
                "MOCK_NESTED_PACKAGES": "github.com/goplus/spx/v4/cmd/ispx",
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
        nested = [f"{module}/cmd/ispx", f"{module}/cmd/ispx/helper"]
        for cover in (False, True):
            with self.subTest(cover=cover):
                completed = self.run_script(
                    [*included, *excluded], *(["--cover"] if cover else []), *arguments,
                    MOCK_NESTED_PACKAGES="\n".join(nested),
                )
                self.assertEqual(completed.returncode, 0, completed.stderr)
                root_coverage = ["-coverpkg=" + ",".join(included)] if cover else []
                nested_coverage = ["-coverpkg=" + ",".join(nested)] if cover else []
                self.assertEqual(self.recorded_calls(), [
                    {"directory": ".", "args": ["list", "./..."]},
                    {"directory": ".", "args": ["test", *root_coverage, *arguments, *included]},
                    {"directory": "cmd/ispx", "args": ["list", "./..."]},
                    {"directory": "cmd/ispx", "args": ["test", *nested_coverage, *arguments, *nested]},
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

    def test_nested_discovery_cannot_run_a_partial_selection(self):
        for environment, code in (({"MOCK_FAIL_CALL": "3"}, 7), ({"MOCK_NESTED_PACKAGES": ""}, 1)):
            with self.subTest(environment=environment):
                completed = self.run_script(["github.com/goplus/spx/v4"], "--cover", **environment)
                self.assertEqual(completed.returncode, code, completed.stderr)
                calls = self.recorded_calls()
                self.assertEqual(len(calls), 3)
                self.assertEqual(calls[-1], {"directory": "cmd/ispx", "args": ["list", "./..."]})

    # Expected report paths and package selections, independent of recorded calls.
    coverage_suites = {
        "macos": {
            "coverage-buildctl-macos.out": [
                "./internal/cmd/buildctl/shared", "./internal/cmd/buildctl/tool", "./internal/cmd/buildctl/runtimecmd",
            ],
            "coverage-command-macos.out": ["./cmd/spx/internal/command"],
        },
        "windows": {
            "coverage-buildctl-windows.out": ["./internal/cmd/buildctl/shared", "./internal/cmd/buildctl/tool"],
            "coverage-engine-windows.out": ["./internal/cmd/buildctl/engine"],
            "coverage-runtimebundle-windows.out": ["./internal/runtimebundle"],
        },
        "web": {
            "coverage-web.out": [
                "./internal/gdengine/binding/web", "./internal/gdengine/impl", "./internal/engine/wrap",
                "./pkg/ispx", "./internal/coroutine",
            ],
        },
        "web-process": {
            "coverage-web-process.out": ["./internal/cmd/buildctl/engine", "./internal/processsupervisor", "./pkg/launcher"],
        },
        "web-interpreter": {"cmd/ispx/coverage-web.out": ["./..."]},
        "tools": {"coverage-tools.out": ["./.github/cmd/..."]},
        "codegen": {"internal/cmd/codegen/coverage.out": ["./..."]},
        "native": {
            "coverage.out": ["github.com/goplus/spx/v4"],
            "cmd/ispx/coverage.out": ["github.com/goplus/spx/v4/cmd/ispx"],
        },
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
                "MOCK_GOHOSTARCH": "amd64",
                "MOCK_GOOS": {"macos": "darwin", "windows": "windows"}.get(suite, "linux"),
                **environment,
            },
        )

    def test_coverage_suites_select_targets_and_report_every_profile(self):
        all_profiles = [profile for reports in self.coverage_suites.values() for profile in reports]
        self.assertEqual(len(all_profiles), len(set(all_profiles)))
        for suite, reports in self.coverage_suites.items():
            with self.subTest(suite=suite):
                result = self.run_coverage_suite(suite)
                self.assertEqual(result.returncode, 0, result.stderr)
                calls = self.recorded_calls()
                tests = [call for call in calls if call["args"][0] == "test"]
                self.assertEqual(len(tests), len(reports))
                for call, (profile, packages) in zip(tests, reports.items()):
                    args = call["args"]
                    self.assertEqual(call["directory"], Path(profile).parent.as_posix())
                    web = suite.startswith("web")
                    self.assertEqual((call["GOOS"], call["GOARCH"]), ("js", "wasm") if web else ("", ""))
                    options = [f"-coverprofile={Path(profile).name}", "-covermode=atomic"]
                    if web:
                        options.append('-exec="/mock/go root/lib/wasm/go_js_wasm_exec"')
                    if suite in ("native", "codegen"):
                        options.append("-coverpkg=" + ",".join(packages))
                    if suite in ("native", "tools", "macos", "windows"):
                        options.append("-v")
                    if suite in ("macos", "windows", "web-process"):
                        self.assertIn("-run", args)
                        for option in options:
                            self.assertIn(option, args)
                        self.assertEqual([arg for arg in args if arg.startswith(("./", "github.com/"))], packages)
                        self.assertEqual([arg for arg in args if arg.startswith("-coverprofile=")], [options[0]])
                        self.assertFalse(any(arg.endswith(".go") for arg in args))
                    else:
                        # Full suites must not accept extra flags that narrow or skip tests.
                        self.assertCountEqual(args, ["test", *options, *packages])
                compact = {
                    "directory": ".", "args": ["run", "./.github/cmd/compact-coverage", *reports],
                    "GOOS": {"macos": "darwin", "windows": "windows"}.get(suite, "linux"), "GOARCH": "amd64",
                }
                self.assertEqual([call for call in calls if call["args"][0] == "run"], [compact])
                self.assertEqual(calls[-1], compact)
                self.assertEqual(self.output.read_text().splitlines(), ["coverage<<COVERAGE_PATHS", *reports, "COVERAGE_PATHS"])
                actual = [profile.relative_to(self.root).as_posix() for profile in self.root.rglob("coverage*.out")]
                self.assertCountEqual(actual, reports)

    def test_windows_coverage_includes_cache_security_tests(self):
        result = self.run_coverage_suite("windows")
        self.assertEqual(result.returncode, 0, result.stderr)
        cache_test, = [call for call in self.recorded_calls() if "./internal/runtimebundle" in call["args"]]
        args = cache_test["args"]
        for name in ("TestCacheDefaultPermissionsProtectNestedFilesAndRepair",
                     "TestCacheUsesConfiguredWindowsPermissions",
                     "TestWindowsPrivateDACLIsAppliedAtCreation",
                     "TestAcquireCreatesProtectedWindowsCache",
                     "TestVerifyPrivateSecurityDescriptorUsesBinarySIDs",
                     "TestVerifyPrivateSecurityDescriptorAcceptsExpectedForms",
                     "TestVerifyPrivateSecurityDescriptorRejectsUnexpectedACL",
                     "TestVerifyPrivateSecurityDescriptorRejectsMissingDACL"):
            with self.subTest(name=name):
                self.assertRegex(name, args[args.index("-run") + 1])

    def test_web_compaction_uses_host_with_exported_target(self):
        for suite in ("web", "web-process", "web-interpreter"):
            with self.subTest(suite=suite):
                result = self.run_coverage_suite(
                    suite, GOOS="js", GOARCH="wasm", MOCK_GOHOSTARCH="arm64",
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                calls = self.recorded_calls()
                for call in calls:
                    if call["args"][0] == "test":
                        self.assertEqual((call["GOOS"], call["GOARCH"]), ("js", "wasm"))
                self.assertEqual(calls[-1]["args"][0], "run")
                self.assertEqual((calls[-1]["GOOS"], calls[-1]["GOARCH"]), ("linux", "arm64"))
                self.assertTrue(self.output.exists())

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
