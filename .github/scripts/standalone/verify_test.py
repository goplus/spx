#!/usr/bin/env python3
"""Exercise the real standalone verifier with local executable fixtures."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest
import zipfile


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / ".github/scripts/standalone/verify.sh"
PLATFORM = ROOT / ".github/scripts/standalone/platform.sh"


class StandaloneVerificationTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="spx standalone verify ")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.tools = self.root / "tools"
        self.tools.mkdir()
        (self.root / "tmp").mkdir()
        project = self.root / "test/CI"
        project.mkdir(parents=True)
        (project / "main.spx").write_text("fixture")
        self.original_gopath = self.root / "installed"
        (self.original_gopath / "bin").mkdir(parents=True)
        (self.original_gopath / "bin/external-runtime").write_text("installed runtime")
        self.trace = self.root / "smoke-ran"
        self.write_tool("file", '#!/bin/bash\nprintf "%s\\n" "$BINARY_DESCRIPTION"\n')
        self.write_tool("go", """#!/bin/bash
set -eu
case "$*" in
  'env GOMODCACHE') printf '%s\\n' "$EXPECTED_MODULE_CACHE" ;;
  'env GOCACHE') printf '%s\\n' "$EXPECTED_BUILD_CACHE" ;;
  'run ./.github/cmd/runtime-version --spx-version') printf 'v4.0.0\\n' ;;
  *) echo "unexpected go call: $*" >&2; exit 90 ;;
esac
""")
        self.env = {
            **os.environ,
            "PATH": f"{self.tools}{os.pathsep}{os.environ['PATH']}",
            "GOPATH": str(self.original_gopath),
            "GOBIN": str(self.original_gopath / "bin"),
            "GOWORK": "unrelated-workspace",
            "EXPECTED_MODULE_CACHE": str(self.root / "module-cache"),
            "EXPECTED_BUILD_CACHE": str(self.root / "build-cache"),
            "ORIGINAL_GOPATH": str(self.original_gopath),
            "RUNNER_TEMP": str(self.root / "tmp"),
            "PACKAGE_FILE": "standalone.zip",
            "PLATFORM": "linux",
            "PACKAGE_ARCH": "x64",
            "RUNNER_OS_CONTEXT": "Linux",
            "RUNNER_ARCH_CONTEXT": "X64",
            "BINARY_DESCRIPTION": "ELF 64-bit LSB executable, x86-64",
            "SMOKE_TRACE": str(self.trace),
            "SMOKE_EXIT": "0",
            "SMOKE_MARKER": "true",
        }

    def write_tool(self, name, content):
        path = self.tools / name
        path.write_text(content)
        path.chmod(0o755)

    def package(self, script, name="spx"):
        with zipfile.ZipFile(self.root / "standalone.zip", "w") as archive:
            archive.writestr(name, script)
            # A sibling runtime must not mask missing embedded assets either.
            archive.writestr("external-runtime", "archive sibling runtime")

    def verify(self, **environment):
        return subprocess.run(
            ["bash", str(VERIFY)], cwd=self.root,
            env={**self.env, **environment}, text=True, capture_output=True,
            timeout=20,
        )

    def successful_binary(self):
        return """#!/bin/bash
set -eu
[ "$GOPATH" != "$ORIGINAL_GOPATH" ]
[ "$GOBIN" = "$GOPATH/bin" ]
[ "$GOMODCACHE" = "$EXPECTED_MODULE_CACHE" ]
[ "$GOCACHE" = "$EXPECTED_BUILD_CACHE" ]
[ "$GOWORK" = off ]
[ ! -e "$GOPATH/bin/external-runtime" ]
[ ! -e "$(dirname "$0")/external-runtime" ]
[ -d "$XDG_CACHE_HOME" ]
[ "$LOCALAPPDATA" = "$XDG_CACHE_HOME" ]
project="${2#--path=}"
[ -f "$project/go.mod" ]
grep -Fq 'replace github.com/goplus/spx/v4 => "' "$project/go.mod"
printf '%s\\n' "$GOPATH" > "$SMOKE_TRACE"
if [ "$SMOKE_MARKER" = true ]; then printf 'SPX_CI_TEST_OK\\n'; fi
exit "$SMOKE_EXIT"
"""

    def test_native_smoke_isolates_runtime_and_preserves_compiler_caches(self):
        self.package(self.successful_binary())
        result = self.verify()
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertTrue(self.trace.exists())
        isolated_gopath = Path(self.trace.read_text().strip())
        self.assertTrue(isolated_gopath.is_relative_to(self.root / "tmp"))
        self.assertFalse(isolated_gopath.exists(), "temporary verification state leaked")
        self.assertTrue((self.original_gopath / "bin/external-runtime").exists())

    def test_binary_without_embedded_runtime_cannot_use_installed_or_sibling_assets(self):
        self.package("""#!/bin/bash
for directory in "$GOPATH/bin" "$(dirname "$0")"; do
  if [ -f "$directory/external-runtime" ]; then
    echo SPX_CI_TEST_OK
    exit 0
  fi
done
echo 'no embedded or external runtime' >&2
exit 19
""")
        result = self.verify()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("no embedded or external runtime", result.stdout)
        self.assertIn("success marker not found", result.stderr)

    def test_success_marker_and_existing_windows_shutdown_policy(self):
        for platform, status, marker, success in (
            ("linux", "0", "false", False),
            ("linux", "7", "true", False),
            ("windows", "7", "true", True),
            ("windows", "7", "false", False),
        ):
            with self.subTest(platform=platform, status=status, marker=marker):
                self.package(self.successful_binary(), "spx.exe" if platform == "windows" else "spx")
                result = self.verify(
                    PLATFORM=platform, RUNNER_OS_CONTEXT=platform,
                    BINARY_DESCRIPTION="PE32+ executable x86-64" if platform == "windows" else self.env["BINARY_DESCRIPTION"],
                    SMOKE_EXIT=status, SMOKE_MARKER=marker,
                )
                self.assertEqual(result.returncode == 0, success, result.stderr + result.stdout)

    def test_non_native_package_is_checked_without_executing_it(self):
        self.package(self.successful_binary())
        result = self.verify(RUNNER_ARCH_CONTEXT="ARM64")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertIn("Skipping smoke test on non-native runner", result.stdout)
        self.assertFalse(self.trace.exists())

    def test_wrong_architecture_fails_before_smoke(self):
        self.package(self.successful_binary())
        result = self.verify(BINARY_DESCRIPTION="ELF 64-bit ARM aarch64")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("architecture mismatch", result.stderr)
        self.assertFalse(self.trace.exists())


class BinaryArchitectureTest(unittest.TestCase):
    def test_linux_and_windows_architecture_patterns(self):
        targets = (
            ("linux", "x64", "ELF 64-bit LSB executable, x86-64"),
            ("linux", "x86", "ELF 32-bit LSB executable, Intel 80386"),
            ("linux", "arm64", "ELF 64-bit LSB executable, ARM aarch64"),
            ("windows", "x64", "PE32+ executable (console) x86-64"),
            ("windows", "x86", "PE32 executable (console) Intel 80386"),
        )
        script = 'source "$LIBRARY"\nfile() { printf "%s\\n" "$DESCRIPTION"; }\nassert_binary_arch binary "$PLATFORM" "$ARCH"'
        for platform, arch, expected in targets:
            for description in [*(target[2] for target in targets), "data"]:
                with self.subTest(platform=platform, arch=arch, description=description):
                    result = subprocess.run(["bash", "-c", script], capture_output=True, text=True, env={
                        **os.environ, "LIBRARY": str(PLATFORM), "PLATFORM": platform,
                        "ARCH": arch, "DESCRIPTION": description,
                    })
                    self.assertEqual(result.returncode == 0, description == expected, result.stderr)

    def test_macos_requires_the_requested_architecture(self):
        script = 'source "$LIBRARY"\nfile() { echo Mach-O; }\nlipo() { echo "$ARCHS"; }\nassert_binary_arch binary macos "$ARCH"'
        for arch, archs, success in (("x64", "x86_64", True), ("arm64", "arm64", True),
                                      ("arm64", "x86_64 arm64", True), ("x64", "arm64", False)):
            with self.subTest(arch=arch, archs=archs):
                result = subprocess.run(["bash", "-c", script], capture_output=True, text=True, env={
                    **os.environ, "LIBRARY": str(PLATFORM), "ARCH": arch, "ARCHS": archs,
                })
                self.assertEqual(result.returncode == 0, success, result.stderr)


if __name__ == "__main__":
    unittest.main()
