#!/usr/bin/env python3
"""Exercise action scripts without downloading toolchains or running APT."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


from test_support import load_workflow


ROOT = Path(__file__).resolve().parents[3]


class ActionScriptTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="spx action test ")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.bin = self.root / "tools"
        self.bin.mkdir()
        self.trace = self.root / "trace.jsonl"
        self.env = {
            **os.environ,
            "PATH": f"{self.bin}{os.pathsep}{os.environ['PATH']}",
            "TRACE": str(self.trace),
        }

    def tool(self, name, body):
        path = self.bin / name
        path.write_text("#!/usr/bin/env python3\n" + body)
        path.chmod(0o755)
        return path

    def run_script(self, script, **environment):
        return subprocess.run(
            ["bash", str(script)], cwd=self.root, env={**self.env, **environment},
            text=True, capture_output=True, timeout=20,
        )

    def calls(self):
        return [json.loads(line) for line in self.trace.read_text().splitlines()] if self.trace.exists() else []


class BuildctlSetupTest(ActionScriptTest):
    def setUp(self):
        super().setUp()
        self.script = self.root / ".github/scripts/setup/build_buildctl.sh"
        for relative in (".github/scripts/setup/build_buildctl.sh", "cmd/internal/macos_go_toolchain.sh"):
            target = self.root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / relative, target)
        self.output = self.root / "output"
        self.github_env = self.root / "env"
        self.env.update(GITHUB_OUTPUT=str(self.output), GITHUB_ENV=str(self.github_env), GOWORK="external.work")
        self.tool("go", '''import json, os, pathlib, sys
args = sys.argv[1:]
if args == ["env", "GOHOSTOS"]:
    print("linux")
elif args == ["env", "GOEXE"]:
    print(os.environ.get("GOEXE", ""))
elif args[0] == "build":
    with open(os.environ["TRACE"], "a") as trace:
        trace.write(json.dumps({"args": args, "cwd": os.getcwd(), "gowork": os.environ["GOWORK"]}) + "\\n")
    if os.environ.get("BUILD_FAIL"):
        sys.exit(17)
    pathlib.Path(args[2]).write_text("built from this checkout")
else:
    sys.exit(91)
''')

    def test_builds_current_checkout_and_exposes_exact_native_path(self):
        stale = self.tool("stale-buildctl", 'raise SystemExit("stale executable must not be reused")\n')
        for extension in ("", ".exe"):
            with self.subTest(extension=extension):
                self.output.write_text("")
                result = self.run_script(self.script, GOEXE=extension, BUILDCTL=str(stale))
                self.assertEqual(result.returncode, 0, result.stderr)
                executable = self.root / f".bin/buildctl{extension}"
                self.assertEqual(executable.read_text(), "built from this checkout")
                self.assertEqual(self.output.read_text(), f"path={executable}\n")
                self.assertFalse(self.github_env.exists(), "setup must not change the caller's job environment")
                self.assertEqual(self.calls()[-1], {
                    "args": ["build", "-o", str(executable), "./internal/cmd/buildctl"],
                    "cwd": str(self.root), "gowork": "off",
                })

    def test_build_failure_does_not_publish_a_stale_path(self):
        result = self.run_script(self.script, BUILD_FAIL="1")
        self.assertEqual(result.returncode, 17, result.stderr)
        self.assertFalse(self.output.exists())
        self.assertFalse(self.github_env.exists())


class LinuxDependenciesTest(ActionScriptTest):
    def setUp(self):
        super().setUp()
        self.script = ROOT / ".github/scripts/setup/install_linux_dependencies.sh"
        self.env.pop("APT_PROFILE", None)
        self.tool("sudo", '''import json, os, pathlib, sys
args = sys.argv[1:]
trace = pathlib.Path(os.environ["TRACE"])
previous = [json.loads(line) for line in trace.read_text().splitlines()] if trace.exists() else []
operation = "update" if "update" in args else "install"
with trace.open("a") as stream:
    stream.write(json.dumps({"operation": operation, "args": args}) + "\\n")
attempt = sum(item["operation"] == operation for item in previous) + 1
if attempt <= int(os.environ.get(operation.upper() + "_FAILURES", "0")):
    sys.exit(17)
''')
        self.tool("sleep", '''import json, os, sys
with open(os.environ["TRACE"], "a") as stream:
    stream.write(json.dumps({"operation": "sleep", "args": sys.argv[1:]}) + "\\n")
''')

    def test_profiles_preserve_package_order_and_installation_options(self):
        for profile, expected in (
            ("web", ["gcc", "libgl1-mesa-dev", "libegl1-mesa-dev", "libgles2-mesa-dev", "libx11-dev", "xorg-dev", "libasound2-dev", "libopenal-dev", "binaryen", "brotli", "unzip"]),
            ("native", ["gcc", "libgl1-mesa-dev", "libegl1-mesa-dev", "libgles2-mesa-dev", "libx11-dev", "xorg-dev", "libasound2-dev", "libopenal-dev", "unzip"]),
        ):
            with self.subTest(profile=profile):
                self.trace.write_text("")
                result = self.run_script(self.script, APT_PROFILE=profile)
                self.assertEqual(result.returncode, 0, result.stderr)
                update, install = self.calls()
                self.assertEqual(update["args"][:4], ["timeout", "--signal=TERM", "--kill-after=15s", "120s"])
                self.assertEqual(install["args"][:6], ["env", "DEBIAN_FRONTEND=noninteractive", "timeout", "--signal=TERM", "--kill-after=15s", "300s"])
                self.assertEqual(install["args"][install["args"].index("--yes") + 1:], expected)
                for call in (update, install):
                    for option in ("Acquire::Retries=3", "Acquire::http::Timeout=30", "Acquire::https::Timeout=30"):
                        self.assertIn(option, call["args"])
                self.assertIn("DPkg::Lock::Timeout=60", install["args"])

    def test_retries_are_bounded_and_update_failure_prevents_install(self):
        for update_failures, install_failures, success, operations in (
            (1, 1, True, ["update", "sleep", "update", "install", "sleep", "install"]),
            (2, 0, False, ["update", "sleep", "update"]),
            (0, 2, False, ["update", "install", "sleep", "install"]),
        ):
            with self.subTest(update_failures=update_failures, install_failures=install_failures):
                self.trace.write_text("")
                result = self.run_script(self.script, APT_PROFILE="native", UPDATE_FAILURES=str(update_failures), INSTALL_FAILURES=str(install_failures))
                self.assertEqual(result.returncode, 0 if success else 17, result.stderr)
                self.assertEqual([call["operation"] for call in self.calls()], operations)
                self.assertTrue(all(call["args"] == ["10"] for call in self.calls() if call["operation"] == "sleep"))

    def test_missing_or_unknown_profile_fails_before_apt(self):
        for inputs in ({}, {"APT_PROFILE": ""}, {"APT_PROFILE": "unknown"}, {"APT_PROFILE": "web,native"}):
            with self.subTest(inputs=inputs):
                result = self.run_script(self.script, **inputs)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertEqual(self.calls(), [])

    def test_workflow_callers_supply_the_required_profile(self):
        action_path = ".github/actions/install-linux-dependencies"
        action = load_workflow(ROOT / action_path / "action.yml")
        self.assertEqual(set(action["inputs"]), {"profile"})
        self.assertEqual(action["inputs"]["profile"]["required"], "true")
        callers = [
            (path, step)
            for path in (ROOT / ".github/workflows").glob("*.yml")
            for job in load_workflow(path)["jobs"].values()
            for step in job.get("steps", [])
            if step.get("uses") == "./" + action_path
        ]
        self.assertTrue(callers)
        for path, step in callers:
            with self.subTest(workflow=path.name):
                self.assertEqual(set(step["with"]), {"profile"})
                self.assertIn(step["with"]["profile"], ("web", "native"))


class ProductActionsTest(ActionScriptTest):
    def test_products_set_up_their_own_toolchain_and_consume_buildctl_output(self):
        for product in ("build-web-bundle", "package-standalone"):
            with self.subTest(product=product):
                steps = load_workflow(ROOT / f".github/actions/{product}/action.yml")["runs"]["steps"]
                toolchains = [i for i, step in enumerate(steps) if step.get("uses") == "./.github/actions/setup-toolchain"]
                builders = [i for i, step in enumerate(steps) if step.get("uses") == "./.github/actions/setup-buildctl"]
                consumers = [i for i, step in enumerate(steps) if "BUILDCTL" in step.get("env", {})]
                self.assertEqual((len(toolchains), len(builders), len(consumers)), (1, 1, 1))
                self.assertLess(toolchains[0], builders[0])
                self.assertLess(builders[0], consumers[0])
                self.assertEqual(steps[consumers[0]]["env"]["BUILDCTL"], "${{ steps." + steps[builders[0]]["id"] + ".outputs.path }}")

    def test_web_execution_passes_explicit_product_profile_and_optional_assets(self):
        action = load_workflow(ROOT / ".github/actions/build-web-bundle/action.yml")
        step = next(step for step in action["runs"]["steps"] if "WEB_MODE" in step.get("env", {}))
        buildctl = self.tool("buildctl", '''import json, os, sys
with open(os.environ["TRACE"], "a") as trace:
    trace.write(json.dumps({"args": sys.argv[1:], "gowork": os.environ["GOWORK"], "profile": os.environ["SPX_ISPX_PROFILE"]}) + "\\n")
''')
        script = self.root / "build-web.sh"
        script.write_text(step["run"])
        for assets in ("", str(self.root / "engine assets")):
            with self.subTest(assets=assets):
                result = self.run_script(script, BUILDCTL=str(buildctl), WEB_MODE="worker", ENGINE_ASSET_DIR=assets,
                                         GOWORK=step["env"]["GOWORK"], SPX_ISPX_PROFILE=step["env"]["SPX_ISPX_PROFILE"])
                self.assertEqual(result.returncode, 0, result.stderr)
                expected = ["setup", "web", "--mode", "worker", "--export-bundle"]
                if assets:
                    expected += ["--asset-dir", assets]
                self.assertEqual(self.calls()[-1], {"args": expected, "gowork": "off", "profile": "base"})


if __name__ == "__main__":
    unittest.main()
