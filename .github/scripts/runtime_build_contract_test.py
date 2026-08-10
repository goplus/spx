#!/usr/bin/env python3

import copy
import contextlib
import hashlib
import io
import json
import tempfile
import unittest
from pathlib import Path

import runtime_build_contract as contract


REPO_ROOT = Path(__file__).resolve().parents[2]
LOCK_PATH = REPO_ROOT / "internal" / "release" / "runtime.lock.json"


class RuntimeBuildContractTest(unittest.TestCase):
    def setUp(self):
        self.lock = contract.load_runtime_lock(LOCK_PATH)

    def write_json(self, directory, name, value):
        path = Path(directory) / name
        path.write_text(json.dumps(value), encoding="utf-8")
        return path

    def test_toolchain_outputs_are_stable(self):
        outputs = contract.resolve_toolchain(self.lock, require_android_ndk_alias=True)
        self.assertEqual(
            outputs,
            {
                "go-version": "1.25.8",
                "xgo-version": "1.7.5",
                "scons-version": "4.8.1",
                "emsdk-version": "3.1.62",
                "android-ndk-version": "23.2.8568313",
                "android-ndk-installer-alias": "r23c",
                "jdk-version": "17",
            },
        )

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
