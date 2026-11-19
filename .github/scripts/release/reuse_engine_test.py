#!/usr/bin/env python3

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import reuse_engine


class ReuseEngineTest(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        previous = Path.cwd()
        os.chdir(directory.name)
        self.addCleanup(os.chdir, previous)
        self.env = {"ENGINE_TAG": "runtime-v4.0.3", "REPOSITORY": "goplus/spx",
                    "RUNTIME_MANIFEST": "runtime-manifest.json"}
        self.metadata = {"runtime_version": "4.0.3", "assets": [
            {"name": "editor-linux-x86_64.zip"}, {"name": "linux-x86_64.zip"},
            {"name": "spx-runtime-assets.zip"},
        ]}
        self.calls = []
        self.download = mock.patch.object(reuse_engine, "download_release", side_effect=self.download_source).start()
        self.verify = mock.patch.object(reuse_engine.subprocess, "run", side_effect=self.verify_source).start()
        self.addCleanup(mock.patch.stopall)

    def download_source(self, tag, repository, directory, *patterns):
        self.calls.append(patterns)
        self.assertEqual((tag, repository), (self.env["ENGINE_TAG"], self.env["REPOSITORY"]))
        if patterns == (self.env["RUNTIME_MANIFEST"],):
            (directory / self.env["RUNTIME_MANIFEST"]).write_text(json.dumps(self.metadata))
            return
        self.assertEqual(patterns, ("editor-linux-x86_64.zip", "linux-x86_64.zip"))
        for name in patterns:
            (directory / name).write_text(name)

    def verify_source(self, command, check):
        self.calls.append("verify")
        self.assertTrue(check)
        self.assertFalse(Path("reused-engine").exists())
        self.assertEqual(command[:3], ("go", "run", "./.github/cmd/runtime-manifest"))
        self.assertIn("--verify-engine-manifest", command)
        self.assertEqual(command[-2:], ("--revision", "HEAD"))
        source = Path(command[command.index("--asset-directory") + 1])
        self.assertEqual({path.name for path in source.iterdir()},
                         {"runtime-manifest.json", "editor-linux-x86_64.zip", "linux-x86_64.zip"})

    def test_verified_engines_and_original_manifest_are_staged_without_pack(self):
        reuse_engine.reuse_engine(self.env)
        self.assertEqual(self.calls, [("runtime-manifest.json",), ("editor-linux-x86_64.zip", "linux-x86_64.zip"), "verify"])
        destination = Path("reused-engine")
        self.assertEqual({path.name for path in destination.iterdir()},
                         {"editor-linux-x86_64.zip", "linux-x86_64.zip", "engine-source-manifest.json"})
        self.assertEqual(json.loads((destination / "engine-source-manifest.json").read_text()), self.metadata)
        self.assertEqual((destination / "linux-x86_64.zip").read_text(), "linux-x86_64.zip")

    def test_failed_verification_stages_nothing(self):
        self.verify.side_effect = subprocess.CalledProcessError(1, "verify")
        with self.assertRaises(subprocess.CalledProcessError):
            reuse_engine.reuse_engine(self.env)
        self.assertFalse(Path("reused-engine").exists())

    def test_failed_download_does_not_verify_or_stage(self):
        self.download.side_effect = subprocess.CalledProcessError(1, "gh")
        with self.assertRaises(subprocess.CalledProcessError):
            reuse_engine.reuse_engine(self.env)
        self.verify.assert_not_called()
        self.assertFalse(Path("reused-engine").exists())

    def test_manifest_must_belong_to_requested_source_tag(self):
        self.metadata["runtime_version"] = "4.0.4"
        with self.assertRaisesRegex(ValueError, "selected release tag"):
            reuse_engine.reuse_engine(self.env)
        self.download.assert_called_once()
        self.verify.assert_not_called()
        self.assertFalse(Path("reused-engine").exists())

    def test_unusable_inventory_does_not_download_engines(self):
        for names in ([], ["spx-runtime-assets.zip"], ["*.zip"], ["unrelated.txt"], [None]):
            with self.subTest(names=names):
                self.download.reset_mock()
                self.metadata["assets"] = [{"name": name} for name in names]
                with self.assertRaisesRegex(ValueError, "literal engine ZIP names"):
                    reuse_engine.reuse_engine(self.env)
                self.download.assert_called_once()
                self.verify.assert_not_called()
        self.assertFalse(Path("reused-engine").exists())

    def test_failed_engine_download_does_not_verify_or_stage(self):
        def download_manifest_then_fail(tag, repository, directory, *patterns):
            if self.download.call_count == 2:
                raise subprocess.CalledProcessError(1, "gh")
            self.download_source(tag, repository, directory, *patterns)

        self.download.side_effect = download_manifest_then_fail
        with self.assertRaises(subprocess.CalledProcessError):
            reuse_engine.reuse_engine(self.env)
        self.assertEqual(self.download.call_count, 2)
        self.verify.assert_not_called()
        self.assertFalse(Path("reused-engine").exists())

    def test_existing_destination_is_not_overwritten(self):
        Path("reused-engine").mkdir()
        self.verify.side_effect = None
        with self.assertRaises(FileExistsError):
            reuse_engine.reuse_engine(self.env)


if __name__ == "__main__":
    unittest.main()
