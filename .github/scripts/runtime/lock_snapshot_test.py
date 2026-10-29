#!/usr/bin/env python3

import copy
import contextlib
import io
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from runtime import build_contract as contract
from runtime import lock_snapshot
from test_support import REPO_ROOT, SCRIPTS


LOCK_PATH = REPO_ROOT / "internal/release/runtime.lock.json"


def accept_godot_ancestry(
    repository,
    godot_ref,
    godot_commit,
    allow_premerge_candidate=False,
):
    del repository, allow_premerge_candidate
    full_ref = godot_ref if godot_ref.startswith("refs/") else f"refs/heads/{godot_ref}"
    return {
        "status": "ancestor",
        "ref": full_ref,
        "ref_commit": godot_commit,
    }


class RuntimeLockSnapshotTest(unittest.TestCase):
    def setUp(self):
        self.lock = contract.load_runtime_lock(LOCK_PATH)

    def test_cli_resolves_its_contract_without_pythonpath(self):
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run(
                [sys.executable, "-I", str(SCRIPTS / "runtime/lock_snapshot.py"), "check",
                 "--lock", str(LOCK_PATH),
                 "--snapshot-directory", str(REPO_ROOT / "internal/release/runtime_locks")],
                cwd=directory, capture_output=True, text=True,
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Current runtime lock snapshot matches", result.stdout)

    def test_current_runtime_lock_snapshot_is_canonical(self):
        lock = contract.load_runtime_lock(LOCK_PATH)
        self.assertEqual(LOCK_PATH.read_bytes(), lock_snapshot.canonical_runtime_lock(lock))

    def test_snapshot_sync_only_writes_the_current_version(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()
            historical_path = snapshot_directory / "1.0.0.json"
            historical_path.write_bytes(b"historical\n")

            snapshot_path, changed = lock_snapshot.sync_snapshot(lock_path, snapshot_directory)

            self.assertTrue(changed)
            self.assertEqual(snapshot_path.name, f"{self.lock['runtime_version']}.json")
            self.assertEqual(snapshot_path.read_bytes(), lock_path.read_bytes())
            self.assertEqual(historical_path.read_bytes(), b"historical\n")
            self.assertEqual(lock_snapshot.check_snapshot(lock_path, snapshot_directory), snapshot_path)

    def test_snapshot_sync_requires_explicit_unpublished_update(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()
            snapshot_path = snapshot_directory / f"{self.lock['runtime_version']}.json"
            snapshot_path.write_bytes(b"existing published bytes\n")

            with self.assertRaisesRegex(lock_snapshot.SnapshotError, "refusing to replace"):
                lock_snapshot.sync_snapshot(lock_path, snapshot_directory)
            self.assertEqual(snapshot_path.read_bytes(), b"existing published bytes\n")

            updated_path, changed = lock_snapshot.sync_snapshot(
                lock_path,
                snapshot_directory,
                allow_unpublished_update=True,
            )
            self.assertTrue(changed)
            self.assertEqual(updated_path.read_bytes(), lock_path.read_bytes())

    def test_pin_godot_updates_lock_and_current_snapshot_together(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()
            snapshot_path = snapshot_directory / f"{self.lock['runtime_version']}.json"
            snapshot_path.write_bytes(LOCK_PATH.read_bytes())
            historical_path = snapshot_directory / "1.0.0.json"
            historical_path.write_bytes(b"historical\n")
            original_lock = lock_path.read_bytes()

            with self.assertRaisesRegex(lock_snapshot.SnapshotError, "refusing to replace"):
                lock_snapshot.pin_godot(
                    lock_path,
                    snapshot_directory,
                    "b" * 40,
                    "spx4.4.1-next",
                )
            self.assertEqual(lock_path.read_bytes(), original_lock)
            self.assertEqual(snapshot_path.read_bytes(), original_lock)

            updated_path, changed, ancestry = lock_snapshot.pin_godot(
                lock_path,
                snapshot_directory,
                "b" * 40,
                "spx4.4.1-next",
                allow_unpublished_update=True,
                ancestry_verifier=accept_godot_ancestry,
            )
            self.assertTrue(changed)
            self.assertEqual(ancestry["status"], "ancestor")
            updated_lock = contract.load_runtime_lock(lock_path)
            self.assertEqual(updated_lock["godot"]["commit"], "b" * 40)
            self.assertEqual(updated_lock["godot"]["ref"], "spx4.4.1-next")
            self.assertEqual(updated_path.read_bytes(), lock_path.read_bytes())
            self.assertEqual(historical_path.read_bytes(), b"historical\n")

    def test_pin_godot_creates_a_missing_current_snapshot_without_override(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()
            historical_path = snapshot_directory / "1.0.0.json"
            historical_path.write_bytes(b"historical\n")

            snapshot_path, changed, ancestry = lock_snapshot.pin_godot(
                lock_path,
                snapshot_directory,
                "c" * 40,
                "spx4.4.1-next",
                ancestry_verifier=accept_godot_ancestry,
            )

            self.assertTrue(changed)
            self.assertEqual(ancestry["status"], "ancestor")
            self.assertEqual(snapshot_path.read_bytes(), lock_path.read_bytes())
            self.assertEqual(historical_path.read_bytes(), b"historical\n")

    def test_pin_godot_retains_the_current_ref_when_omitted(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()
            seen_refs = []

            def capture_ref(repository, godot_ref, godot_commit, **options):
                del repository, godot_commit, options
                seen_refs.append(godot_ref)
                return {
                    "status": "ancestor",
                    "ref": f"refs/heads/{godot_ref}",
                    "ref_commit": "f" * 40,
                }

            lock_snapshot.pin_godot(
                lock_path,
                snapshot_directory,
                "f" * 40,
                ancestry_verifier=capture_ref,
            )

            self.assertEqual(seen_refs, [self.lock["godot"]["ref"]])
            self.assertEqual(
                contract.load_runtime_lock(lock_path)["godot"]["ref"],
                self.lock["godot"]["ref"],
            )

    def test_pin_godot_rejects_invalid_inputs_without_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()
            snapshot_path = snapshot_directory / f"{self.lock['runtime_version']}.json"
            snapshot_path.write_bytes(LOCK_PATH.read_bytes())
            original_lock = lock_path.read_bytes()

            with self.assertRaises(contract.ContractError):
                lock_snapshot.pin_godot(
                    lock_path,
                    snapshot_directory,
                    "not-a-commit",
                    "spx4.4.1\nrefs/heads/injected",
                    allow_unpublished_update=True,
                )
            self.assertEqual(lock_path.read_bytes(), original_lock)
            self.assertEqual(snapshot_path.read_bytes(), original_lock)

    def test_pin_godot_verifies_before_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()
            original_lock = lock_path.read_bytes()
            calls = []

            def reject_ancestry(repository, godot_ref, godot_commit, **options):
                calls.append((repository, godot_ref, godot_commit, options))
                raise lock_snapshot.SnapshotError("remote unavailable")

            with self.assertRaisesRegex(lock_snapshot.SnapshotError, "remote unavailable"):
                lock_snapshot.pin_godot(
                    lock_path,
                    snapshot_directory,
                    "d" * 40,
                    "spx4.4.1-next",
                    ancestry_verifier=reject_ancestry,
                )

            self.assertEqual(
                calls,
                [
                    (
                        lock_snapshot.EXPECTED_GODOT_REPOSITORY,
                        "spx4.4.1-next",
                        "d" * 40,
                        {"allow_premerge_candidate": False},
                    )
                ],
            )
            self.assertEqual(lock_path.read_bytes(), original_lock)
            self.assertFalse(
                (snapshot_directory / f"{self.lock['runtime_version']}.json").exists()
            )

    def test_pin_godot_passes_explicit_premerge_policy(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock_path = root / "runtime.lock.json"
            lock_path.write_bytes(LOCK_PATH.read_bytes())
            snapshot_directory = root / "runtime_locks"
            snapshot_directory.mkdir()

            def confirm_premerge(repository, godot_ref, godot_commit, **options):
                self.assertEqual(repository, lock_snapshot.EXPECTED_GODOT_REPOSITORY)
                self.assertTrue(options["allow_premerge_candidate"])
                return {
                    "status": "premerge",
                    "ref": f"refs/heads/{godot_ref}",
                    "ref_commit": self.lock["godot"]["commit"],
                }

            snapshot_path, changed, ancestry = lock_snapshot.pin_godot(
                lock_path,
                snapshot_directory,
                "e" * 40,
                "spx4.4.1-next",
                allow_premerge_candidate=True,
                ancestry_verifier=confirm_premerge,
            )

            self.assertTrue(changed)
            self.assertEqual(ancestry["status"], "premerge")
            self.assertEqual(snapshot_path.read_bytes(), lock_path.read_bytes())

    def test_pin_godot_cli_maps_concise_policies(self):
        ancestry = {
            "status": "ancestor",
            "ref": "refs/heads/spx4.4.1",
            "ref_commit": "f" * 40,
        }
        cases = (
            ((), False, False),
            (("--unpublished",), True, False),
            (("--premerge",), True, True),
        )
        for flags, allow_unpublished, allow_premerge in cases:
            with self.subTest(flags=flags), mock.patch.object(
                lock_snapshot,
                "pin_godot",
                return_value=(Path("snapshot.json"), True, ancestry),
            ) as pin, contextlib.redirect_stdout(io.StringIO()):
                return_code = lock_snapshot.main(
                    [
                        "pin-godot",
                        "f" * 40,
                        *flags,
                        "--lock",
                        str(LOCK_PATH),
                    ]
                )

            self.assertEqual(return_code, 0)
            pin.assert_called_once_with(
                LOCK_PATH,
                lock_snapshot.DEFAULT_SNAPSHOT_DIRECTORY,
                "f" * 40,
                None,
                allow_unpublished,
                allow_premerge,
            )

    def test_pin_godot_cli_requires_commit_and_exclusive_policy(self):
        invalid_commands = (
            ["pin-godot"],
            ["pin-godot", "f" * 40, "--unpublished", "--premerge"],
        )
        for command in invalid_commands:
            with self.subTest(command=command), contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit) as raised:
                    lock_snapshot.main(command)
            self.assertEqual(raised.exception.code, 2)

    def test_godot_ancestry_rejects_untrusted_lock_repository(self):
        with tempfile.TemporaryDirectory() as directory:
            lock = copy.deepcopy(self.lock)
            lock["godot"]["repository"] = "https://github.com/example/godot.git"
            lock_path = Path(directory) / "runtime.lock.json"
            lock_path.write_bytes(lock_snapshot.canonical_runtime_lock(lock))
            verifier = mock.Mock()

            with self.assertRaisesRegex(
                lock_snapshot.SnapshotError,
                "not the trusted canonical repository",
            ):
                lock_snapshot.verify_locked_godot_ancestry(
                    lock_path,
                    ancestry_verifier=verifier,
                )

            verifier.assert_not_called()

    def test_godot_ancestry_cli_emits_structured_outputs(self):
        result = {
            "status": "premerge",
            "ref": "refs/heads/spx4.4.1",
            "ref_commit": "a" * 40,
        }
        with tempfile.TemporaryDirectory() as directory:
            output_path = Path(directory) / "output"
            with mock.patch.object(
                lock_snapshot,
                "verify_locked_godot_ancestry",
                return_value=result,
            ) as verifier, contextlib.redirect_stdout(io.StringIO()) as stdout:
                return_code = lock_snapshot.main(
                    [
                        "verify-godot",
                        "--lock",
                        str(LOCK_PATH),
                        "--allow-premerge-candidate",
                        "--github-output",
                        str(output_path),
                    ]
                )

            self.assertEqual(return_code, 0)
            verifier.assert_called_once_with(LOCK_PATH, True)
            self.assertIn("publication is blocked", stdout.getvalue())
            self.assertEqual(
                output_path.read_text(encoding="utf-8"),
                "godot_ancestry_status=premerge\n"
                "godot_ref=refs/heads/spx4.4.1\n"
                f"godot_ref_commit={'a' * 40}\n",
            )

    def test_godot_ancestry_cli_rejects_candidate_flag_for_check(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit) as raised:
                lock_snapshot.main(["check", "--allow-premerge-candidate"])
        self.assertEqual(raised.exception.code, 2)

    def run_git(self, cwd, *arguments):
        result = subprocess.run(
            ["git", *arguments],
            cwd=cwd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
        )
        if result.returncode != 0:
            self.fail(
                f"git {' '.join(arguments)} failed ({result.returncode}): "
                f"{result.stderr.strip() or result.stdout.strip()}"
            )
        return result.stdout.strip()

    def create_godot_remote(self, root):
        work = Path(root) / "work"
        remote = Path(root) / "remote.git"
        work.mkdir()
        self.run_git(work, "init", "--quiet", "--initial-branch=stable")
        self.run_git(work, "config", "user.name", "SPX Test")
        self.run_git(work, "config", "user.email", "spx-test@example.com")

        source = work / "source.txt"
        source.write_text("ancestor\n", encoding="utf-8")
        self.run_git(work, "add", "source.txt")
        self.run_git(work, "commit", "--quiet", "-m", "ancestor")
        ancestor = self.run_git(work, "rev-parse", "HEAD")

        source.write_text("stable\n", encoding="utf-8")
        self.run_git(work, "commit", "--quiet", "-am", "stable")
        stable = self.run_git(work, "rev-parse", "HEAD")
        self.run_git(work, "tag", "-a", "stable-tag", "-m", "stable tag", stable)

        self.run_git(work, "checkout", "--quiet", "-b", "candidate", stable)
        source.write_text("candidate\n", encoding="utf-8")
        self.run_git(work, "commit", "--quiet", "-am", "candidate")
        candidate = self.run_git(work, "rev-parse", "HEAD")

        self.run_git(work, "checkout", "--quiet", "-b", "divergent", ancestor)
        source.write_text("divergent\n", encoding="utf-8")
        self.run_git(work, "commit", "--quiet", "-am", "divergent")
        divergent = self.run_git(work, "rev-parse", "HEAD")

        self.run_git(root, "init", "--bare", "--quiet", remote)
        self.run_git(remote, "config", "uploadpack.allowFilter", "true")
        self.run_git(remote, "config", "uploadpack.allowAnySHA1InWant", "true")
        self.run_git(work, "remote", "add", "origin", str(remote))
        self.run_git(
            work,
            "push",
            "--quiet",
            "origin",
            "refs/heads/stable",
            "refs/heads/candidate",
            "refs/heads/divergent",
            "refs/tags/stable-tag",
        )
        return work, remote, ancestor, stable, candidate, divergent

    def test_godot_ancestry_uses_exact_bounded_remote_ref_history(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            work, remote, ancestor, stable, candidate, divergent = self.create_godot_remote(root)

            result = lock_snapshot.verify_godot_ancestry(remote, "stable", ancestor)
            self.assertEqual(
                result,
                {
                    "status": "ancestor",
                    "ref": "refs/heads/stable",
                    "ref_commit": stable,
                },
            )

            tag_result = lock_snapshot.verify_godot_ancestry(
                remote,
                "refs/tags/stable-tag",
                ancestor,
            )
            self.assertEqual(tag_result["status"], "ancestor")
            self.assertEqual(tag_result["ref_commit"], stable)

            with self.assertRaisesRegex(lock_snapshot.SnapshotError, "not an ancestor"):
                lock_snapshot.verify_godot_ancestry(remote, "stable", candidate)
            premerge = lock_snapshot.verify_godot_ancestry(
                remote,
                "stable",
                candidate,
                allow_premerge_candidate=True,
            )
            self.assertEqual(premerge["status"], "premerge")
            self.assertEqual(premerge["ref_commit"], stable)

            with self.assertRaisesRegex(lock_snapshot.SnapshotError, "ancestry is unknown"):
                lock_snapshot.verify_godot_ancestry(
                    remote,
                    "stable",
                    divergent,
                    allow_premerge_candidate=True,
                )

            with self.assertRaises(lock_snapshot.SnapshotError):
                lock_snapshot.verify_godot_ancestry(
                    remote,
                    "stable",
                    "f" * 40,
                    allow_premerge_candidate=True,
                )

            self.run_git(work, "tag", "stable", stable)
            self.run_git(work, "push", "--quiet", "origin", "refs/tags/stable")
            with self.assertRaisesRegex(lock_snapshot.SnapshotError, "ambiguous"):
                lock_snapshot.verify_godot_ancestry(remote, "stable", stable)

            full_ref_result = lock_snapshot.verify_godot_ancestry(
                remote,
                "refs/heads/stable",
                stable,
            )
            self.assertEqual(full_ref_result["status"], "ancestor")

    def test_godot_ref_rejects_unsafe_or_unsupported_namespaces(self):
        runner = lock_snapshot.GitRunner()
        for godot_ref in (
            "refs/pull/1/head",
            "refs/remotes/origin/stable",
            "refs/heads/stable:refs/heads/injected",
            "stable*",
            "stable^",
            "-upload-pack=evil",
        ):
            with self.subTest(godot_ref=godot_ref):
                with self.assertRaises(lock_snapshot.SnapshotError):
                    lock_snapshot.remote_ref_candidates(godot_ref, runner)


if __name__ == "__main__":
    unittest.main()
