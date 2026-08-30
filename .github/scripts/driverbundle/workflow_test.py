#!/usr/bin/env python3

import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
ACTION = ROOT / "actions" / "driver-bundle" / "action.yml"


class DriverBundleActionTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.action = ACTION.read_text(encoding="utf-8")

    def test_inputs_are_the_complete_packaging_contract(self):
        inputs = self.action.split("inputs:\n", 1)[1].split("runs:\n", 1)[0]
        names = re.findall(r"^  ([a-z][a-z0-9-]*):\s*$", inputs, re.MULTILINE)
        self.assertEqual(
            names,
            ["engine", "pack", "bridge", "output", "descriptor", "goos", "goarch"],
        )
        self.assertEqual(inputs.count("required: true"), len(names))

    def test_composite_action_wires_package_then_verify(self):
        self.assertIn("using: composite", self.action)
        self.assertEqual(self.action.count("shell: bash"), 2)

        for mapping in (
            "ENGINE_PATH: ${{ inputs.engine }}",
            "PACK_PATH: ${{ inputs.pack }}",
            "BRIDGE_PATH: ${{ inputs.bridge }}",
            "OUTPUT_PATH: ${{ inputs.output }}",
            "DESCRIPTOR_PATH: ${{ inputs.descriptor }}",
            "TARGET_GOOS: ${{ inputs.goos }}",
            "TARGET_GOARCH: ${{ inputs.goarch }}",
        ):
            self.assertIn(mapping, self.action)

        commands = (
            "go run ./.github/scripts/driverbundle package",
            "go run ./.github/scripts/driverbundle verify",
        )
        for command in commands:
            self.assertEqual(self.action.count(command), 1)

        package_step = self.action.split(commands[0], 1)[1].split(commands[1], 1)[0]
        for argument in (
            '--engine "$ENGINE_PATH"',
            '--pack "$PACK_PATH"',
            '--bridge "$BRIDGE_PATH"',
            '--output "$OUTPUT_PATH"',
            '--descriptor "$DESCRIPTOR_PATH"',
            '--goos "$TARGET_GOOS"',
            '--goarch "$TARGET_GOARCH"',
        ):
            self.assertIn(argument, package_step)

        verify_step = self.action.split(commands[1], 1)[1]
        for argument in ('--output "$OUTPUT_PATH"', '--descriptor "$DESCRIPTOR_PATH"'):
            self.assertIn(argument, verify_step)

    def test_action_is_not_coupled_to_release_activation(self):
        for activation in ("standalone/prepare", "release_driver", "gh release"):
            self.assertNotIn(activation, self.action)


if __name__ == "__main__":
    unittest.main()
