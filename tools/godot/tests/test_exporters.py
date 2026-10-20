"""Native Godot exporter checks: python3 -m unittest discover -s tools/godot/tests.

Requires Godot 4.3+ on PATH (or GODOT_BIN). All project/cache/output files
are created in a temporary directory; no editor import or GPU is required.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class ExporterTests(unittest.TestCase):
    def test_native_exporter_contracts(self):
        godot = shutil.which(os.environ.get("GODOT_BIN", "godot"))
        if godot is None:
            self.skipTest("Godot 4.3+ is required; set GODOT_BIN")
        source = Path(__file__).resolve().parent
        with tempfile.TemporaryDirectory(prefix="spx-exporter-") as directory:
            project = Path(directory)
            (project / "project.godot").write_text('config_version=5\n', encoding="utf-8")
            shutil.copytree(source.parent / "addons", project / "addons")
            shutil.copyfile(source / "exporter_test.gd", project / "exporter_test.gd")
            environment = dict(os.environ, HOME=directory, XDG_CACHE_HOME=directory,
                               XDG_DATA_HOME=directory, XDG_CONFIG_HOME=directory)
            result = subprocess.run(
                [godot, "--headless", "--path", directory, "--script", "exporter_test.gd"],
                capture_output=True, text=True, timeout=60, env=environment,
            )
            output = result.stdout + result.stderr
            self.assertEqual(result.returncode, 0, output)
            # Godot can report script errors without returning a failing exit code.
            self.assertNotIn("ERROR:", output, output)
            self.assertIn("Exporter contracts passed", output, output)


if __name__ == "__main__":
    unittest.main()
