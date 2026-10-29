#!/usr/bin/env python3

import copy
import contextlib
import hashlib
import io
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from runtime import build_contract as contract
from test_support import REPO_ROOT, SCRIPTS


LOCK_PATH = REPO_ROOT / "internal/release/runtime.lock.json"


class RuntimeBuildContractTest(unittest.TestCase):
    def setUp(self):
        self.lock = contract.load_runtime_lock(LOCK_PATH)

    def test_pinned_godot_sparse_checkout_needs_only_contract_and_lock(self):
        # The pinned Godot action checks out exactly these two files before Go
        # or the SPX module is available. An importing compatibility wrapper
        # would break this bootstrap even if a full-checkout test passed.
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            script = root / ".github/scripts/runtime/build_contract.py"
            lock = root / "internal/release/runtime.lock.json"
            script.parent.mkdir(parents=True)
            lock.parent.mkdir(parents=True)
            shutil.copyfile(SCRIPTS / "runtime/build_contract.py", script)
            shutil.copyfile(LOCK_PATH, lock)
            cases = [("toolchain", None), *(("source", scope) for scope in ("native", "web", "android"))]
            for command, scope in cases:
                with self.subTest(scope=scope, command=command):
                    output = root / f"{scope}-{command}.out"
                    args = [command, "--lock", str(lock), "--github-output", str(output)]
                    if command == "source":
                        args += ["--runtime-version", self.lock["runtime_version"],
                                 "--runtime-abi", str(self.lock["runtime_abi"]),
                                 "--godot-origin", self.lock["godot"]["repository"],
                                 "--godot-commit", self.lock["godot"]["commit"],
                                 "--toolchain-scope", scope]
                    result = subprocess.run([sys.executable, "-I", str(script), *args],
                                            cwd=root, capture_output=True, text=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertIn(f"go-version={self.lock['toolchain']['go']}\n", output.read_text())
                    if command == "source":
                        self.assertRegex(output.read_text(), r"(?m)^engine-toolchain-sha256=[0-9a-f]{64}$")
                    self.assertEqual(result.stdout, self.lock["module"]["path"] + "\n"
                                     if command == "source" else "")

    def write_json(self, directory, name, value):
        path = Path(directory) / name
        path.write_text(json.dumps(value), encoding="utf-8")
        return path

    def test_toolchain_outputs_are_stable(self):
        outputs = contract.resolve_toolchain(self.lock, require_android_ndk_alias=True)
        self.assertEqual(
            outputs,
            {
                "go-version": "1.26.5",
                "xgo-version": "1.7.5",
                "scons-version": "4.8.1",
                "emsdk-version": "3.1.62",
                "android-ndk-version": "23.2.8568313",
                "android-ndk-installer-alias": "r23c",
                "jdk-version": "17",
            },
        )

    def test_describe_lock_outputs_checkout_and_toolchain_identity(self):
        outputs = contract.describe_lock(self.lock)
        self.assertEqual(outputs["runtime-version"], self.lock["runtime_version"])
        self.assertEqual(outputs["runtime-abi"], str(self.lock["runtime_abi"]))
        self.assertEqual(outputs["godot-repository"], "goplus/godot")
        self.assertEqual(outputs["godot-ref"], self.lock["godot"]["ref"])
        self.assertEqual(outputs["godot-commit"], self.lock["godot"]["commit"])
        self.assertEqual(outputs["module-relative-path"], self.lock["module"]["path"])
        self.assertEqual(outputs["scons-version"], self.lock["toolchain"]["scons"])
        self.assertEqual(outputs["emsdk-version"], self.lock["toolchain"]["emsdk"])

    def test_engine_toolchain_digest_is_scoped(self):
        outputs = contract.resolve_toolchain(self.lock, toolchain_scope="android")
        payload = (
            "spx-engine-toolchain-android-v1\n"
            "scons=4.8.1\n"
            "jdk=17\n"
            "android_ndk=23.2.8568313\n"
        )
        self.assertEqual(
            outputs["engine-toolchain-sha256"],
            hashlib.sha256(payload.encode("utf-8")).hexdigest(),
        )

    def test_engine_toolchain_scope_field_matrix(self):
        scopes = tuple(contract.ENGINE_TOOLCHAIN_SCOPES)
        baseline = {
            scope: contract.resolve_toolchain(self.lock, toolchain_scope=scope)["engine-toolchain-sha256"]
            for scope in scopes
        }
        affected_scopes = {
            "scons": {"native", "web", "android"},
            "emsdk": {"web"},
            "jdk": {"android"},
            "android_ndk": {"android"},
            "go": set(),
            "xgo": set(),
        }
        for field, affected in affected_scopes.items():
            with self.subTest(field=field):
                lock = copy.deepcopy(self.lock)
                lock["toolchain"][field] += ".changed"
                for scope in scopes:
                    digest = contract.resolve_toolchain(lock, toolchain_scope=scope)["engine-toolchain-sha256"]
                    self.assertEqual(digest != baseline[scope], scope in affected, (field, scope))

    def test_unknown_ndk_alias_only_blocks_explicit_android_requirement(self):
        lock = copy.deepcopy(self.lock)
        lock["toolchain"]["android_ndk"] = "99.0.0"
        outputs = contract.resolve_toolchain(lock)
        self.assertEqual(outputs["android-ndk-version"], "99.0.0")
        self.assertEqual(outputs["android-ndk-installer-alias"], "")
        with self.assertRaisesRegex(contract.ContractError, "no validated setup-ndk alias"):
            contract.resolve_toolchain(lock, require_android_ndk_alias=True)

    def test_source_accepts_equivalent_github_origin(self):
        module_path = contract.validate_source(
            self.lock,
            "v" + self.lock["runtime_version"],
            str(self.lock["runtime_abi"]),
            "git@github.com:goplus/godot.git",
            self.lock["godot"]["commit"],
        )
        self.assertEqual(module_path, "godot_modules/spx")

    def test_source_rejects_identity_drift(self):
        with self.assertRaisesRegex(contract.ContractError, "does not match godot.commit"):
            contract.validate_source(
                self.lock,
                self.lock["runtime_version"],
                str(self.lock["runtime_abi"]),
                self.lock["godot"]["repository"],
                "f" * 40,
            )

    def test_module_path_rejects_repository_root(self):
        with self.assertRaisesRegex(contract.ContractError, "normalized relative path"):
            contract.validate_module_path(".")

    def test_module_path_rejects_empty_and_git_revision_injection(self):
        for module_path in ("", "../spx", "godot_modules/spx:refs/heads/main", "godot_modules/spx^{tree}"):
            with self.subTest(module_path=module_path):
                with self.assertRaises(contract.ContractError):
                    contract.validate_module_path(module_path)

    def test_profile_outputs_merge_common_flags(self):
        profile = {
            "schema": 1,
            "common": ["builtin_brotli=no", "deprecated=no"],
            "editor_release": ["debug_symbols=no"],
            "template_release": ["optimize=size"],
        }
        with tempfile.TemporaryDirectory() as directory:
            path = self.write_json(directory, "profile.json", profile)
            outputs = contract.load_scons_profile(path)
        self.assertEqual(outputs["common-flags"], "builtin_brotli=no deprecated=no")
        self.assertEqual(
            outputs["editor-release-flags"],
            "builtin_brotli=no deprecated=no debug_symbols=no",
        )
        self.assertEqual(
            outputs["template-release-flags"],
            "builtin_brotli=no deprecated=no optimize=size",
        )

    def test_profile_rejects_orchestration_flags(self):
        profile = {
            "schema": 1,
            "common": ["platform=linux"],
            "editor_release": [],
            "template_release": [],
        }
        with tempfile.TemporaryDirectory() as directory:
            path = self.write_json(directory, "profile.json", profile)
            with self.assertRaisesRegex(contract.ContractError, "orchestration-owned"):
                contract.load_scons_profile(path)

    def test_profile_rejects_malformed_tokens_and_duplicate_keys(self):
        cases = (
            (None, "must be an array"),
            ([None], "non-empty token"),
            ([""], "non-empty token"),
            (["optimize=space size"], "without whitespace"),
            (["optimize=size\x00"], "without whitespace"),
            (["optimize"], "single key=value"),
            (["optimize=size=extra"], "single key=value"),
            (["optimize="], "valid non-empty key and value"),
            (["-optimize=size"], "valid non-empty key and value"),
            (["optimize=size", "optimize=speed"], "duplicate key"),
        )
        for group in contract.PROFILE_GROUPS:
            for tokens, message in cases:
                with self.subTest(group=group, tokens=tokens), tempfile.TemporaryDirectory() as directory:
                    profile = {"schema": 1, "common": [], "editor_release": [], "template_release": []}
                    profile[group] = tokens
                    path = self.write_json(directory, "profile.json", profile)
                    with self.assertRaisesRegex(contract.ContractError, message):
                        contract.load_scons_profile(path)

    def test_profile_allows_variant_overrides_only_between_siblings(self):
        profile = {
            "schema": 1, "common": [],
            "editor_release": ["optimize=speed"],
            "template_release": ["optimize=size"],
        }
        with tempfile.TemporaryDirectory() as directory:
            path = self.write_json(directory, "profile.json", profile)
            self.assertEqual(contract.load_scons_profile(path), {
                "common-flags": "", "editor-release-flags": "optimize=speed",
                "template-release-flags": "optimize=size",
            })
            for variant in ("editor_release", "template_release"):
                with self.subTest(variant=variant):
                    conflicting = {"schema": 1, "common": ["optimize=size"], "editor_release": [], "template_release": []}
                    conflicting[variant] = ["optimize=speed"]
                    path = self.write_json(directory, "profile.json", conflicting)
                    with self.assertRaisesRegex(contract.ContractError, f"common and {variant} contain duplicate"):
                        contract.load_scons_profile(path)

    def test_lock_rejects_boolean_schema_and_abi(self):
        for field in ("schema", "runtime_abi"):
            with self.subTest(field=field):
                lock = copy.deepcopy(self.lock)
                lock[field] = True
                with self.assertRaisesRegex(contract.ContractError, "integer"):
                    contract.validate_runtime_lock(lock)

    def test_repository_normalization_preserves_the_github_trust_boundary(self):
        for origin in (
            "https://GITHUB.COM/GoPlus/Godot.git", "git@github.com:goplus/godot.git",
            "ssh://git@github.com:22/goplus/godot", "github.com/goplus/godot/",
        ):
            with self.subTest(origin=origin):
                self.assertEqual(contract.normalize_github_repository(origin, "origin"), "goplus/godot")
        for origin in (
            "https://github.com.evil.test/goplus/godot.git",
            "https://github.com:444/goplus/godot.git",
            "https://github.com/goplus/godot?ref=main",
            "https://github.com/goplus/godot#main",
            "ssh://root@github.com/goplus/godot",
            "https://github.com/goplus/godot/tree/main",
            "https://github.com/goplus/%2e%2e", "goplus/godot\n",
        ):
            with self.subTest(origin=origin), self.assertRaises(contract.ContractError):
                contract.normalize_github_repository(origin, "origin")

    def test_profile_cli_emits_all_flag_groups(self):
        profile = {
            "schema": 1,
            "common": ["builtin_brotli=no"],
            "editor_release": ["debug_symbols=no"],
            "template_release": ["optimize=size"],
        }
        with tempfile.TemporaryDirectory() as directory:
            profile_path = self.write_json(directory, "profile.json", profile)
            output_path = Path(directory) / "output"
            self.assertEqual(
                contract.main(
                    [
                        "profile",
                        "--profile",
                        str(profile_path),
                        "--github-output",
                        str(output_path),
                    ]
                ),
                0,
            )
            self.assertEqual(
                output_path.read_text(encoding="utf-8"),
                "common-flags=builtin_brotli=no\n"
                "editor-release-flags=builtin_brotli=no debug_symbols=no\n"
                "template-release-flags=builtin_brotli=no optimize=size\n",
            )

    def test_lock_rejects_duplicate_fields(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "lock.json"
            path.write_text('{"schema": 1, "schema": 1}', encoding="utf-8")
            with self.assertRaisesRegex(contract.ContractError, "duplicate JSON field"):
                contract.load_runtime_lock(path)

    def test_lock_requires_canonical_godot_repository(self):
        lock = copy.deepcopy(self.lock)
        lock["godot"]["repository"] = "goplus/godot"
        with tempfile.TemporaryDirectory() as directory:
            path = self.write_json(directory, "lock.json", lock)
            with self.assertRaisesRegex(contract.ContractError, "canonical URL"):
                contract.load_runtime_lock(path)

    def test_lock_requires_sorted_assets(self):
        lock = copy.deepcopy(self.lock)
        lock["required_assets"][0], lock["required_assets"][1] = (
            lock["required_assets"][1],
            lock["required_assets"][0],
        )
        with tempfile.TemporaryDirectory() as directory:
            path = self.write_json(directory, "lock.json", lock)
            with self.assertRaisesRegex(contract.ContractError, "sorted by name"):
                contract.load_runtime_lock(path)

    def test_github_outputs_are_single_line(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "output"
            contract.write_github_outputs(path, {"one": "1", "two": "2"})
            self.assertEqual(path.read_text(encoding="utf-8"), "one=1\ntwo=2\n")
            with self.assertRaisesRegex(contract.ContractError, "single line"):
                contract.write_github_outputs(path, {"bad": "one\ntwo"})

    def test_toolchain_cli_requires_ndk_alias_only_when_requested(self):
        lock = copy.deepcopy(self.lock)
        lock["toolchain"]["android_ndk"] = "99.0.0"
        with tempfile.TemporaryDirectory() as directory:
            lock_path = self.write_json(directory, "lock.json", lock)
            output_path = Path(directory) / "output"
            base_args = [
                "toolchain",
                "--lock",
                str(lock_path),
                "--github-output",
                str(output_path),
            ]
            self.assertEqual(contract.main(base_args), 0)
            self.assertIn("android-ndk-installer-alias=\n", output_path.read_text(encoding="utf-8"))

            error_output = Path(directory) / "error-output"
            with contextlib.redirect_stderr(io.StringIO()) as stderr:
                return_code = contract.main(
                    base_args[:-1] + [str(error_output), "--require-android-ndk-alias"]
                )
            self.assertEqual(return_code, 1)
            self.assertIn("no validated setup-ndk alias", stderr.getvalue())
            self.assertFalse(error_output.exists())

    def test_describe_cli_emits_lock_identity(self):
        with tempfile.TemporaryDirectory() as directory:
            output_path = Path(directory) / "output"
            self.assertEqual(
                contract.main(
                    [
                        "describe",
                        "--lock",
                        str(LOCK_PATH),
                        "--github-output",
                        str(output_path),
                    ]
                ),
                0,
            )
            output = output_path.read_text(encoding="utf-8")
            self.assertIn(f"runtime-version={self.lock['runtime_version']}\n", output)
            self.assertIn(f"runtime-abi={self.lock['runtime_abi']}\n", output)
            self.assertIn("godot-repository=goplus/godot\n", output)
            self.assertIn(f"godot-commit={self.lock['godot']['commit']}\n", output)
            self.assertIn(f"module-relative-path={self.lock['module']['path']}\n", output)

    def test_source_cli_emits_scoped_digest_and_module_path(self):
        with tempfile.TemporaryDirectory() as directory:
            output_path = Path(directory) / "output"
            with contextlib.redirect_stdout(io.StringIO()) as stdout:
                return_code = contract.main(
                    [
                        "source",
                        "--lock",
                        str(LOCK_PATH),
                        "--github-output",
                        str(output_path),
                        "--runtime-version",
                        self.lock["runtime_version"],
                        "--runtime-abi",
                        str(self.lock["runtime_abi"]),
                        "--godot-origin",
                        "git@github.com:goplus/godot.git",
                        "--godot-commit",
                        self.lock["godot"]["commit"],
                        "--toolchain-scope",
                        "web",
                    ]
                )
            self.assertEqual(return_code, 0)
            self.assertEqual(stdout.getvalue(), "godot_modules/spx\n")
            output = output_path.read_text(encoding="utf-8")
            self.assertIn("module-relative-path=godot_modules/spx\n", output)
            self.assertRegex(output, r"(?m)^engine-toolchain-sha256=[0-9a-f]{64}$")


if __name__ == "__main__":
    unittest.main()
