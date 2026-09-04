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

DRIVER_WORKFLOW = ROOT / "workflows" / "release_driver.yml"
PLATFORM_WORKFLOW = ROOT / "workflows" / "release_driver_platform.yml"
RELEASE_WORKFLOW = ROOT / "workflows" / "release.yml"
PREPARE_ACTION = ROOT / "actions" / "standalone" / "prepare" / "action.yml"


class DriverWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.driver = DRIVER_WORKFLOW.read_text(encoding="utf-8")
        cls.platform = PLATFORM_WORKFLOW.read_text(encoding="utf-8")
        cls.release = RELEASE_WORKFLOW.read_text(encoding="utf-8")
        cls.prepare = PREPARE_ACTION.read_text(encoding="utf-8")

    def test_driver_workflow_builds_assets_for_the_spx_release(self):
        self.assertIn("workflow_call:", self.driver)
        self.assertNotIn("workflow_dispatch:", self.driver)
        self.assertIn("v<semver>", self.driver)
        self.assertIn("spx_version:", self.driver)
        self.assertIn("INPUT_VERSION: ${{ inputs.spx_version }}", self.driver)
        self.assertIn("EXPECTED_VERSION: ${{ steps.release.outputs.release_tag }}", self.driver)
        self.assertIn("spx_version: ${{ steps.release.outputs.release_tag }}", self.driver)
        self.assertIn("args=(assemble", self.driver)
        self.assertIn("spx-driver-assets-", self.driver)
        for obsolete in (
            "driver-v<SPX semver",
            "driver_tag",
            "DRIVER_TAG",
            "gh release upload",
            "gh release edit",
            "release_tag:",
            "Require the published runtime",
            "verify-release",
        ):
            self.assertNotIn(obsolete, self.driver)

    def test_exact_platform_matrix_and_public_assets(self):
        for target in (
            "{goos: darwin, goarch: amd64, runner: macos-15-intel}",
            "{goos: darwin, goarch: arm64, runner: macos-15}",
            "{goos: linux, goarch: amd64, runner: ubuntu-22.04}",
            "{goos: windows, goarch: amd64, runner: windows-latest}",
        ):
            self.assertIn(target, self.driver)
        self.assertIn("driver-manifest.json", self.driver)
        self.assertIn(
            "spx-driver-${{ inputs.goos }}-${{ inputs.goarch }}.zip",
            self.platform,
        )
        self.assertIn("go run ./.github/scripts/driverbundle", self.driver)

    def test_platform_workflow_reuses_prepare_outputs(self):
        self.assertIn("uses: ./.github/actions/standalone/prepare", self.platform)
        self.assertIn("steps.prepare.outputs.engine-path", self.platform)
        self.assertIn("steps.prepare.outputs.pack-path", self.platform)
        self.assertIn("steps.prepare.outputs.bridge-path", self.platform)
        self.assertIn("uses: ./.github/actions/driver-bundle", self.platform)
        self.assertNotIn("actions/standalone/package", self.platform)
        self.assertIn('GOARCH="$(go env GOARCH', self.prepare)
        self.assertIn('gdspx-${GOOS}-${GOARCH}', self.prepare)

    def test_spx_release_collects_driver_assets(self):
        self.assertIn("driver-assets:", self.release)
        self.assertIn("uses: ./.github/workflows/release_driver.yml", self.release)
        self.assertIn("spx_version: ${{ needs.setup.outputs.release_tag }}", self.release)
        self.assertIn("needs.driver-assets.result == 'success'", self.release)
        self.assertIn("spx-driver-assets-${{ needs.setup.outputs.release_tag }}", self.release)
        self.assertIn("Assemble unified SPX release", self.release)
        self.assertIn(
            "required+=(setup assemble publish-runtime driver-assets publish-spx",
            self.release,
        )
        self.assertNotIn("driver_tag", self.release)

    def test_release_has_no_pin_handoff(self):
        for obsolete in (
            "runtime-pin-handoff:",
            "make release-pin",
            "spx-runtime-pin-",
            "spx-driver-pin-",
            "pin handoff",
            "runtime_pin_state",
            "driver_pin_state",
        ):
            self.assertNotIn(obsolete, self.release)

if __name__ == "__main__":
    unittest.main()
