#!/usr/bin/env python3

import contextlib
import io
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import download


class ReleaseDownloadTest(unittest.TestCase):
    def setUp(self):
        stack = contextlib.ExitStack()
        self.addCleanup(stack.close)
        self.run = stack.enter_context(mock.patch.object(download, "run"))
        self.sleep = stack.enter_context(mock.patch.object(download.time, "sleep"))
        stack.enter_context(contextlib.redirect_stderr(io.StringIO()))

    def test_first_attempt_succeeds_without_waiting(self):
        download.download_release("runtime-v4.0.2", "fixture/spx", Path("runtime assets"))

        self.run.assert_called_once()
        command = self.run.call_args.args[0]
        self.assertEqual(tuple(command[:4]), ("gh", "release", "download", "runtime-v4.0.2"))
        self.assertEqual(command[command.index("--repo") + 1], "fixture/spx")
        self.assertEqual(command[command.index("--dir") + 1], "runtime assets")
        self.assertTrue(self.run.call_args.kwargs["check"])
        self.sleep.assert_not_called()

    def test_retry_replaces_partial_asset(self):
        with tempfile.TemporaryDirectory(prefix="spx release download ") as directory:
            asset = Path(directory) / "engine.zip"

            def transfer(command, *, check):
                self.assertTrue(check)
                if not asset.exists():
                    asset.write_bytes(b"partial")
                    raise subprocess.CalledProcessError(1, command)
                if "--clobber" not in command:
                    raise subprocess.CalledProcessError(1, command)
                asset.write_bytes(b"complete asset")

            self.run.side_effect = transfer
            download.download_release("runtime-v4.0.2", "fixture/spx", directory)

            self.assertEqual(asset.read_bytes(), b"complete asset")
            self.assertEqual(self.run.call_count, 2)
            self.sleep.assert_called_once_with(5)

    def test_persistent_failure_is_bounded_and_preserves_last_error(self):
        errors = [subprocess.CalledProcessError(code, ["gh"]) for code in (1, 2, 3)]
        self.run.side_effect = errors

        with self.assertRaises(subprocess.CalledProcessError) as raised:
            download.download_release("runtime-v4.0.2", "fixture/spx", "runtime")

        self.assertIs(raised.exception, errors[-1])
        self.assertEqual(self.run.call_count, 3)
        self.assertEqual(self.sleep.call_args_list, [mock.call(5), mock.call(10)])

    def test_local_errors_and_termination_are_not_retried(self):
        for error in (FileNotFoundError("gh"), subprocess.CalledProcessError(-15, ["gh"])):
            with self.subTest(error=error):
                self.run.reset_mock()
                self.sleep.reset_mock()
                self.run.side_effect = error

                with self.assertRaises(type(error)) as raised:
                    download.download_release("runtime-v4.0.2", "fixture/spx", "runtime")

                self.assertIs(raised.exception, error)
                self.run.assert_called_once()
                self.sleep.assert_not_called()


if __name__ == "__main__":
    unittest.main()
