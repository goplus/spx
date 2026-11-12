"""Native Godot exporter checks: python3 -m unittest discover -s tools/godot/tests.

Requires Godot 4.3+ on PATH (or GODOT_BIN). Projects and outputs are temporary.
Set SPX_TEST_PREVIEW=1 with a rendering display to include PNG pixel checks.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
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


class ExporterCLITests(unittest.TestCase):
    def setUp(self):
        self.godot = shutil.which(os.environ.get("GODOT_BIN", "godot"))
        if self.godot is None:
            self.skipTest("Godot 4.3+ is required; set GODOT_BIN")
        self.source = Path(__file__).resolve().parent
        temporary = tempfile.TemporaryDirectory(prefix="spx export project ")
        self.addCleanup(temporary.cleanup)
        self.project = Path(temporary.name)
        (self.project / "project.godot").write_text(
            'config_version=5\n[rendering]\nrenderer/rendering_method="gl_compatibility"\n',
            encoding="utf-8",
        )
        shutil.copyfile(self.source / "export_fixture.gd", self.project / "fixture.gd")
        self.run_godot("--headless", "--script", "fixture.gd")

    def run_process(self, command, expected=0, environment=None):
        result = subprocess.run(command, capture_output=True, text=True, timeout=60,
                                env=environment, cwd=self.source)
        output = result.stdout + result.stderr
        self.assertEqual(result.returncode, expected, output)
        if expected == 0:
            self.assertNotIn("ERROR:", output, output)
        return output

    def run_godot(self, *arguments, expected=0):
        return self.run_process([self.godot, "--path", str(self.project), *arguments],
                                expected=expected)

    def install_addon(self):
        shutil.copytree(self.source.parent / "addons", self.project / "addons")

    def assert_exported_data(self):
        output = self.project / "_export" / "terrain"
        tilemap = json.loads((output / "tilemap.json").read_text())
        decorators = json.loads((output / "decorator.json").read_text())
        self.assertEqual(len(tilemap["layers"]), 1)
        self.assertEqual(tilemap["tileset"]["sources"][0]["texture"], "tilemap/tile.tres")
        self.assertEqual(len(decorators["decorators"]), 1)
        self.assertEqual(decorators["decorators"][0]["name"], "Decoration")
        self.assertEqual((output / "tilemap" / "tile.tres").read_bytes(),
                         (self.project / "tile.tres").read_bytes())
        self.assertTrue((output / "decorator" / "decorator.tres").is_file())

    def test_fresh_project_headless_cli(self):
        self.install_addon()
        for options in [[], ["--no-preview"]]:
            with self.subTest(options=options):
                output = self.run_godot("--headless", "--script",
                                        "addons/spx_tilemap_exporter/export_cli.gd", "--",
                                        "--scene", "levels/terrain.tscn", *options)
                self.assertIn("Export Preview: false", output)
                self.assert_exported_data()
                self.assertFalse((self.project / "_export/terrain/preview.png").exists())

    def test_cli_rejects_invalid_arguments_and_missing_scene(self):
        self.install_addon()
        for arguments in [["--scene"], ["--unknown"], ["--scene", "../outside.tscn"],
                          ["--scene", "res://missing.tscn"]]:
            with self.subTest(arguments=arguments):
                self.run_godot("--headless", "--script",
                               "addons/spx_tilemap_exporter/export_cli.gd", "--",
                               *arguments, expected=1)
        self.assertFalse((self.project / "_export").exists())

    def test_python_copy_project_path_and_godot_precedence(self):
        addon = self.project / "addons/spx_tilemap_exporter"
        addon.mkdir(parents=True)
        sentinel = addon / "local-notes.txt"
        sentinel.write_text("keep me", encoding="utf-8")
        environment = dict(os.environ, GODOT_PATH="missing-godot")
        wrapper = self.source.parent / "addons/spx_tilemap_exporter/export.py"
        self.run_process([sys.executable, str(wrapper), "--project", str(self.project),
                          "--copy", "--godot", self.godot, "--scene",
                          "levels/terrain.tscn", "--no-preview"], environment=environment)
        self.assertEqual(sentinel.read_text(), "keep me")
        self.assert_exported_data()
        # Installed invocation finds its project independently of the current directory.
        # Copying the installed addon onto itself is a no-op.
        environment["GODOT_PATH"] = self.godot
        self.run_process([sys.executable, str(addon / "export.py"), "--copy", "--scene",
                          "res://levels/terrain.tscn", "--no-preview"], environment=environment)
        self.assertFalse((self.project / "_export/terrain/preview.png").exists())

    def test_editor_plugin_loads(self):
        self.install_addon()
        with (self.project / "project.godot").open("a") as project:
            project.write('[editor_plugins]\nenabled=PackedStringArray("res://addons/spx_tilemap_exporter/plugin.cfg")\n')
        self.run_godot("--headless", "--editor", "--import")

    @unittest.skipUnless(os.environ.get("SPX_TEST_PREVIEW") == "1",
                         "Set SPX_TEST_PREVIEW=1 with a rendering display to test PNG pixels")
    def test_preview_png_pixels_and_no_preview_switch(self):
        self.install_addon()
        # Test the switch with a real renderer as well as the headless fallback.
        arguments = ["--resolution", "64x64", "--script",
                     "addons/spx_tilemap_exporter/export_cli.gd", "--", "--scene",
                     "levels/terrain.tscn"]
        self.run_godot(*arguments, "--no-preview")
        self.assertFalse((self.project / "_export/terrain/preview.png").exists())
        self.run_godot(*arguments)
        self.assert_exported_data()
        output = self.run_godot("--headless", "--script", "fixture.gd", "--", "--verify-preview")
        self.assertIn("Preview pixels passed", output)


if __name__ == "__main__":
    unittest.main()
