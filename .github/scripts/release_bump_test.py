#!/usr/bin/env python3

import shutil
import tempfile
import unittest
from pathlib import Path

import release_bump
import runtime_build_contract as contract
import runtime_lock_snapshot as lock_snapshot


REPO_ROOT = Path(__file__).resolve().parents[2]


def accept_release_boundary(*_):
    return None


def accept_post_write_validation(*_):
    return None


class ReleaseBumpTest(unittest.TestCase):
    def next_patch(self, version):
        prefix = "v" if version.startswith("v") else ""
        major, minor, patch, _ = release_bump.parse_semver(
            version,
            "fixture version",
            require_spx_major=bool(prefix),
        )
        return f"{prefix}{major}.{minor}.{patch + 1}"

    def fixture(self, directory):
        root = Path(directory)
        lock_path = root / "runtime.lock.json"
        current_path = root / "current_spx_version.go"
        release_meta_path = root / "release_meta.go"
        snapshot_directory = root / "runtime_locks"
        snapshot_directory.mkdir()
        shutil.copyfile(
            REPO_ROOT / "internal/release/runtime.lock.json",
            lock_path,
        )
        shutil.copyfile(
            REPO_ROOT / "internal/release/current_spx_version.go",
            current_path,
        )
        shutil.copyfile(
            REPO_ROOT / "internal/release/release_meta.go",
            release_meta_path,
        )
        lock = contract.load_runtime_lock(lock_path)
        current_spx_version, _ = release_bump.read_current_spx_version(current_path)
        current_snapshot = snapshot_directory / f"{lock['runtime_version']}.json"
        shutil.copyfile(lock_path, current_snapshot)
        historical_snapshot = snapshot_directory / "1.0.0.json"
        historical_snapshot.write_bytes(b"immutable historical snapshot\n")
        return {
            "lock": lock_path,
            "current": current_path,
            "meta": release_meta_path,
            "snapshots": snapshot_directory,
            "current_snapshot": current_snapshot,
            "historical_snapshot": historical_snapshot,
            "current_spx_version": current_spx_version,
            "current_runtime_version": lock["runtime_version"],
            "current_runtime_abi": lock["runtime_abi"],
        }

    def bump(self, paths, spx_version=None, runtime_version=None, runtime_abi=None):
        spx_version = spx_version or self.next_patch(paths["current_spx_version"])
        runtime_version = runtime_version or self.next_patch(
            paths["current_runtime_version"]
        )
        return release_bump.bump_release(
            spx_version,
            runtime_version,
            runtime_abi,
            paths["lock"],
            paths["snapshots"],
            paths["current"],
            paths["meta"],
            accept_release_boundary,
            accept_post_write_validation,
        )

    def test_bump_archives_current_mapping_and_creates_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = self.fixture(directory)
            old_snapshot = paths["current_snapshot"].read_bytes()

            result = self.bump(paths)

            new_spx_version = self.next_patch(paths["current_spx_version"])
            new_runtime_version = self.next_patch(paths["current_runtime_version"])
            self.assertEqual(result["previous_spx_version"], paths["current_spx_version"])
            self.assertEqual(result["spx_version"], new_spx_version)
            self.assertEqual(
                result["previous_runtime_version"], paths["current_runtime_version"]
            )
            self.assertEqual(result["runtime_version"], new_runtime_version)
            updated_lock = contract.load_runtime_lock(paths["lock"])
            self.assertEqual(updated_lock["runtime_version"], new_runtime_version)
            self.assertEqual(updated_lock["runtime_abi"], paths["current_runtime_abi"])
            self.assertEqual(
                paths["current"].read_text(encoding="utf-8").count(
                    f'const currentSPXVersion = "{new_spx_version}"'
                ),
                1,
            )
            self.assertIn(
                f'{{spxVersion: "{paths["current_spx_version"]}", '
                f'runtimeVersion: "{paths["current_runtime_version"]}"}}',
                paths["meta"].read_text(encoding="utf-8"),
            )
            target_snapshot = paths["snapshots"] / f"{new_runtime_version}.json"
            self.assertEqual(target_snapshot.read_bytes(), paths["lock"].read_bytes())
            self.assertEqual(paths["current_snapshot"].read_bytes(), old_snapshot)
            self.assertEqual(
                paths["historical_snapshot"].read_bytes(),
                b"immutable historical snapshot\n",
            )
            self.assertEqual(
                lock_snapshot.check_snapshot(paths["lock"], paths["snapshots"]),
                target_snapshot,
            )

    def test_bump_can_raise_runtime_abi(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = self.fixture(directory)
            new_abi = paths["current_runtime_abi"] + 1
            self.bump(paths, runtime_abi=new_abi)
            self.assertEqual(
                contract.load_runtime_lock(paths["lock"])["runtime_abi"], new_abi
            )

    def test_bump_rejects_non_increasing_versions_without_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            baseline = self.fixture(directory)
            cases = (
                (
                    baseline["current_spx_version"],
                    self.next_patch(baseline["current_runtime_version"]),
                ),
                ("v2.9.0", self.next_patch(baseline["current_runtime_version"])),
                (
                    self.next_patch(baseline["current_spx_version"])[1:],
                    self.next_patch(baseline["current_runtime_version"]),
                ),
                (
                    self.next_patch(baseline["current_spx_version"]),
                    baseline["current_runtime_version"],
                ),
            )
        for spx_version, runtime_version in cases:
            with self.subTest(
                spx_version=spx_version,
                runtime_version=runtime_version,
            ), tempfile.TemporaryDirectory() as directory:
                paths = self.fixture(directory)
                originals = {
                    name: paths[name].read_bytes()
                    for name in ("lock", "current", "meta", "current_snapshot")
                }
                with self.assertRaises(release_bump.ReleaseBumpError):
                    self.bump(paths, spx_version, runtime_version)
                for name, original in originals.items():
                    self.assertEqual(paths[name].read_bytes(), original)
                if runtime_version != paths["current_runtime_version"]:
                    self.assertFalse(
                        (paths["snapshots"] / f"{runtime_version}.json").exists()
                    )

    def test_bump_refuses_existing_target_snapshot_without_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = self.fixture(directory)
            target_runtime = self.next_patch(paths["current_runtime_version"])
            target = paths["snapshots"] / f"{target_runtime}.json"
            target.write_bytes(b"existing immutable bytes\n")
            originals = {
                "lock": paths["lock"].read_bytes(),
                "current": paths["current"].read_bytes(),
                "meta": paths["meta"].read_bytes(),
            }

            with self.assertRaisesRegex(release_bump.ReleaseBumpError, "refusing to replace"):
                self.bump(paths)

            self.assertEqual(target.read_bytes(), b"existing immutable bytes\n")
            for name, original in originals.items():
                self.assertEqual(paths[name].read_bytes(), original)

    def test_bump_rolls_back_when_post_write_validation_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = self.fixture(directory)
            originals = {
                name: paths[name].read_bytes()
                for name in ("lock", "current", "meta", "current_snapshot")
            }
            target_runtime = self.next_patch(paths["current_runtime_version"])
            target_snapshot = paths["snapshots"] / f"{target_runtime}.json"

            def reject_validation(*_):
                raise release_bump.ReleaseBumpError("catalog rejected")

            with self.assertRaisesRegex(release_bump.ReleaseBumpError, "catalog rejected"):
                release_bump.bump_release(
                    self.next_patch(paths["current_spx_version"]),
                    target_runtime,
                    lock_path=paths["lock"],
                    snapshot_directory=paths["snapshots"],
                    current_spx_version_path=paths["current"],
                    release_meta_path=paths["meta"],
                    release_guard=accept_release_boundary,
                    post_write_validator=reject_validation,
                )

            for name, original in originals.items():
                self.assertEqual(paths[name].read_bytes(), original)
            self.assertFalse(target_snapshot.exists())

    def test_bump_rolls_back_when_post_write_validation_is_interrupted(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = self.fixture(directory)
            originals = {
                name: paths[name].read_bytes()
                for name in ("lock", "current", "meta", "current_snapshot")
            }
            target_runtime = self.next_patch(paths["current_runtime_version"])
            target_snapshot = paths["snapshots"] / f"{target_runtime}.json"

            def interrupt_validation(*_):
                raise KeyboardInterrupt

            with self.assertRaises(KeyboardInterrupt):
                release_bump.bump_release(
                    self.next_patch(paths["current_spx_version"]),
                    target_runtime,
                    lock_path=paths["lock"],
                    snapshot_directory=paths["snapshots"],
                    current_spx_version_path=paths["current"],
                    release_meta_path=paths["meta"],
                    release_guard=accept_release_boundary,
                    post_write_validator=interrupt_validation,
                )

            for name, original in originals.items():
                self.assertEqual(paths[name].read_bytes(), original)
            self.assertFalse(target_snapshot.exists())

    def test_semver_prerelease_order_is_canonical(self):
        release_bump.require_newer_version(
            "v3.3.0-rc.1",
            "v3.3.0",
            "SPX version",
            require_spx_major=True,
        )
        with self.assertRaises(release_bump.ReleaseBumpError):
            release_bump.require_newer_version(
                "2.5.0",
                "2.5.0+rebuilt",
                "runtime version",
            )
        with self.assertRaises(release_bump.ReleaseBumpError):
            release_bump.parse_semver("2.5.0-rc.01", "runtime version")

    def test_release_boundary_requires_public_current_and_unused_targets(self):
        repository = "goplus/spx"
        current_spx = "v3.2.0"
        current_runtime = "2.4.0"
        new_spx = "v3.3.0"
        new_runtime = "2.5.0"

        class FakeGitHubClient:
            def __init__(self, values):
                self.values = values

            def get(self, endpoint, allow_missing=False):
                self.assert_missing_allowed = allow_missing
                return self.values.get(endpoint)

        current_values = {}
        for tag in (current_spx, f"runtime-v{current_runtime}"):
            current_values[release_bump.github_release_endpoint(repository, tag)] = {
                "tag_name": tag,
                "draft": False,
            }
            current_values[release_bump.github_tag_endpoint(repository, tag)] = {
                "ref": f"refs/tags/{tag}",
            }
        release_bump.verify_release_boundary(
            repository,
            current_spx,
            current_runtime,
            new_spx,
            new_runtime,
            FakeGitHubClient(current_values),
        )

        draft_values = dict(current_values)
        draft_values[release_bump.github_release_endpoint(repository, current_spx)] = {
            "tag_name": current_spx,
            "draft": True,
        }
        with self.assertRaisesRegex(release_bump.ReleaseBumpError, "not public"):
            release_bump.verify_release_boundary(
                repository,
                current_spx,
                current_runtime,
                new_spx,
                new_runtime,
                FakeGitHubClient(draft_values),
            )

        occupied_values = dict(current_values)
        new_runtime_tag = f"runtime-v{new_runtime}"
        occupied_values[release_bump.github_tag_endpoint(repository, new_runtime_tag)] = {
            "ref": f"refs/tags/{new_runtime_tag}",
        }
        with self.assertRaisesRegex(release_bump.ReleaseBumpError, "already exists"):
            release_bump.verify_release_boundary(
                repository,
                current_spx,
                current_runtime,
                new_spx,
                new_runtime,
                FakeGitHubClient(occupied_values),
            )


if __name__ == "__main__":
    unittest.main()
