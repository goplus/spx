#!/usr/bin/env python3

import contextlib
import hashlib
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

import assemble


class AssemblyTest(unittest.TestCase):
    def setUp(self):
        workspace = tempfile.TemporaryDirectory(prefix="spx release assembly ")
        self.addCleanup(workspace.cleanup)
        self.root = Path(workspace.name)
        original_directory = Path.cwd()
        os.chdir(self.root)
        self.addCleanup(os.chdir, original_directory)
        self.env = {
            "RUNTIME_STATE": "missing", "RUNTIME_TAG": "runtime-v4.0.0",
            "RUNTIME_MANIFEST": "runtime-manifest.json", "REPOSITORY": "fixture/spx",
            "GITHUB_OUTPUT": str(self.root / "output"),
            **{f"RUN_{platform}": "false" for platform in assemble.PRODUCTS},
        }
        self.required = ("engine.zip", "pack.zip")
        assemble.LOCK.parent.mkdir(parents=True)
        assemble.LOCK.write_text(json.dumps({"required_assets": self.required}))
        self.calls = []
        self.fail_command = ""
        patch = mock.patch.object(assemble, "run", side_effect=self.run_external)
        patch.start()
        self.addCleanup(patch.stop)

    def write(self, name, content="asset contents"):
        path = Path(name)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def stage_runtime(self):
        for name in self.required:
            self.write(f"artifacts/runtime/{name}")

    def checksums(self, directory):
        return "".join(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n"
                       for path in sorted(directory.iterdir()) if path.name != "SHA256SUMS")

    def stage_published_runtime(self):
        for name in (*self.required, self.env["RUNTIME_MANIFEST"]):
            self.write(f"published/{name}")
        Path("published/SHA256SUMS").write_text(self.checksums(Path("published")))

    def run_external(self, *args, cwd=None):
        self.calls.append(args)
        if args[0] == "gh":
            operation = "download"
        elif args[0] == "sha256sum":
            operation = "checksums"
        else:
            operation = "verify" if "--verify-manifest" in args else "generate"
        if operation == self.fail_command:
            raise subprocess.CalledProcessError(7, args)
        if operation == "download":
            shutil.copytree("published", args[args.index("--dir") + 1], dirs_exist_ok=True)
        elif operation == "checksums":
            subprocess.run(args, cwd=cwd, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        elif operation == "generate":
            manifest = Path(args[args.index("--output") + 1])
            manifest.write_text('{"generated": true}\n')
            checksums = Path(args[args.index("--checksums") + 1])
            entries = []
            for index, argument in enumerate(args):
                if argument == "--asset":
                    name, path = args[index + 1].split("=", 1)
                    entries.append(f"{hashlib.sha256(Path(path).read_bytes()).hexdigest()}  {name}\n")
            checksums.write_text("".join(sorted(entries)))

    def run_assembly(self):
        with contextlib.redirect_stdout(io.StringIO()):
            assemble.assemble(self.env)

    def assert_no_output(self):
        self.assertFalse(Path(self.env["GITHUB_OUTPUT"]).exists())

    def test_missing_runtime_generates_canonical_manifest_and_product_checksums(self):
        self.stage_runtime()
        self.env.update(RUN_WEB="true", RUN_LINUX="true")
        products = (*assemble.PRODUCTS["WEB"], *assemble.PRODUCTS["LINUX"])
        for name in products:
            self.write(f"artifacts/products/{name}", f"contents of {name}")
        self.run_assembly()
        self.assertEqual(Path("output").read_text(), "product_count=5\n")
        self.assertEqual(Path("dist/product/SHA256SUMS").read_text(), self.checksums(Path("dist/product")))
        self.assertEqual(Path("dist/runtime/SHA256SUMS").read_text(), self.checksums(Path("dist/runtime")))
        generation, = [args for args in self.calls if "--output" in args]
        self.assertEqual(generation[generation.index("--revision") + 1], "HEAD")
        self.assertEqual([generation[index + 1] for index, arg in enumerate(generation) if arg == "--asset"],
                         [f"{name}=dist/runtime/{name}" for name in self.required])
        self.assertEqual(sum("--verify-manifest" in args for args in self.calls), 1)
        self.assertEqual(len(self.calls), 2)

    def test_ready_runtime_reuses_all_verified_assets_without_rebuilding(self):
        self.env["RUNTIME_STATE"] = "ready"
        self.stage_published_runtime()
        self.run_assembly()
        self.assertEqual(Path("output").read_text(), "product_count=0\n")
        self.assertEqual(list(Path("dist/product").iterdir()), [])
        self.assertEqual(len(self.calls), 3)
        self.assertEqual(self.calls[0], ("gh", "release", "download", "runtime-v4.0.0", "--repo", "fixture/spx", "--dir", "dist/runtime"))
        self.assertEqual(self.calls[1], ("sha256sum", "-c", "SHA256SUMS"))
        self.assertIn("--verify-manifest", self.calls[2])

    def test_artifact_selection_fails_closed(self):
        for scenario, message in (("duplicate", "Duplicate asset name"), ("empty", "Empty asset"),
                                   ("missing-runtime", "Missing locked runtime asset"),
                                   ("missing-product", "Missing selected SPX product asset"),
                                   ("unexpected", "Unexpected build artifact"), ("none", "No build artifacts")):
            with self.subTest(scenario=scenario):
                shutil.rmtree("artifacts", ignore_errors=True)
                shutil.rmtree("dist", ignore_errors=True)
                self.env["RUN_LINUX"] = "false"
                self.stage_runtime()
                if scenario == "duplicate":
                    self.write("artifacts/another/engine.zip")
                elif scenario == "empty":
                    self.write("artifacts/runtime/engine.zip", "")
                elif scenario == "missing-runtime":
                    Path("artifacts/runtime/engine.zip").unlink()
                elif scenario == "missing-product":
                    self.env["RUN_LINUX"] = "true"
                elif scenario == "unexpected":
                    self.write("artifacts/runtime/unknown.zip")
                else:
                    shutil.rmtree("artifacts")
                with self.assertRaisesRegex(ValueError, message):
                    self.run_assembly()
                self.assert_no_output()

    def test_reused_runtime_requires_exact_asset_set(self):
        self.env["RUNTIME_STATE"] = "ready"
        for extra in (True, False):
            with self.subTest(extra=extra):
                shutil.rmtree("published", ignore_errors=True)
                shutil.rmtree("dist", ignore_errors=True)
                self.stage_published_runtime()
                if extra:
                    self.write("published/unexpected.zip")
                else:
                    Path("published/engine.zip").unlink()
                Path("published/SHA256SUMS").write_text(self.checksums(Path("published")))
                with self.assertRaisesRegex(ValueError, "Unexpected runtime" if extra else "Runtime release asset is missing"):
                    self.run_assembly()
                self.assert_no_output()

    def test_external_failures_never_publish_product_count(self):
        for state, failures in (("missing", ("generate", "verify")),
                                 ("ready", ("download", "checksums", "verify"))):
            for failure in failures:
                with self.subTest(state=state, failure=failure):
                    shutil.rmtree("artifacts", ignore_errors=True)
                    shutil.rmtree("dist", ignore_errors=True)
                    self.stage_runtime()
                    if state == "ready":
                        shutil.rmtree("artifacts")
                        self.stage_published_runtime()
                    self.env["RUNTIME_STATE"] = state
                    self.fail_command = failure
                    with self.assertRaises(subprocess.CalledProcessError):
                        self.run_assembly()
                    self.assert_no_output()

    def test_corrupt_published_bytes_fail_real_checksum_verification(self):
        self.env["RUNTIME_STATE"] = "ready"
        self.stage_published_runtime()
        self.write("published/engine.zip", "corrupted after checksums")
        with self.assertRaises(subprocess.CalledProcessError):
            self.run_assembly()
        self.assert_no_output()

    def test_invalid_state_fails_at_cli_boundary(self):
        env = {**os.environ, **self.env, "RUNTIME_STATE": "incompatible"}
        result = subprocess.run([sys.executable, assemble.__file__], env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertIn("Invalid runtime release state", result.stderr)
        self.assert_no_output()


if __name__ == "__main__":
    unittest.main()
