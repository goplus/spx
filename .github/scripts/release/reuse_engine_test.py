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

    def download_source(self, tag, repository, directory):
        self.calls.append("download")
        self.assertEqual((tag, repository), (self.env["ENGINE_TAG"], self.env["REPOSITORY"]))
        (directory / self.env["RUNTIME_MANIFEST"]).write_text(json.dumps(self.metadata))
        for asset in self.metadata["assets"]:
            (directory / asset["name"]).write_text(asset["name"])
        (directory / "SHA256SUMS").write_text("not copied")

    def verify_source(self, command, check):
        self.calls.append("verify")
        self.assertTrue(check)
        self.assertFalse(Path("reused-engine").exists())
        self.assertEqual(command[:3], ("go", "run", "./.github/cmd/runtime-manifest"))
        self.assertIn("--verify-engine-manifest", command)
        self.assertEqual(command[-2:], ("--revision", "HEAD"))
        source = Path(command[command.index("--asset-directory") + 1])
        self.assertTrue((source / "editor-linux-x86_64.zip").is_file())

    def test_verified_engines_and_original_manifest_are_staged_without_pack(self):
        reuse_engine.reuse_engine(self.env)
        self.assertEqual(self.calls, ["download", "verify"])
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
        self.assertFalse(Path("reused-engine").exists())

    def test_existing_destination_is_not_overwritten(self):
        Path("reused-engine").mkdir()
        self.verify.side_effect = None
        with self.assertRaises(FileExistsError):
            reuse_engine.reuse_engine(self.env)


if __name__ == "__main__":
    unittest.main()
