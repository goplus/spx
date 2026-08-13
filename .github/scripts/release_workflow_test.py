#!/usr/bin/env python3

import json
import os
import re
import subprocess
import unittest
from pathlib import Path


WORKFLOW_PATH = Path(__file__).resolve().parents[1] / "workflows" / "release.yml"


def job_blocks(text):
    lines = text.splitlines()
    try:
        jobs_line = lines.index("jobs:")
    except ValueError as exc:
        raise AssertionError("release workflow has no top-level jobs mapping") from exc

    starts = [
        (index, match.group(1))
        for index, line in enumerate(lines[jobs_line + 1 :], jobs_line + 1)
        if (match := re.fullmatch(r"  ([a-z0-9-]+):", line))
    ]
    return {
        name: "\n".join(lines[start : starts[position + 1][0]])
        if position + 1 < len(starts)
        else "\n".join(lines[start:])
        for position, (start, name) in enumerate(starts)
    }


def job_needs(block):
    lines = block.splitlines()
    for index, line in enumerate(lines):
        if match := re.fullmatch(r"    needs: \[([^]]*)\]", line):
            return tuple(value.strip() for value in match.group(1).split(","))
        if line == "    needs:":
            values = []
            for item in lines[index + 1 :]:
                if not (match := re.fullmatch(r"      - ([a-z0-9-]+)", item)):
                    break
                values.append(match.group(1))
            return tuple(values)
    return ()


def job_condition(block):
    lines = block.splitlines()
    for index, line in enumerate(lines):
        if not (match := re.fullmatch(r"    if: ?(.*)", line)):
            continue
        if match.group(1) not in (">-", "|", ""):
            value = match.group(1)
        else:
            parts = []
            for item in lines[index + 1 :]:
                if not item.startswith("      "):
                    break
                parts.append(item.strip())
            value = " ".join(parts)
        return "".join(value.replace("${{", "").replace("}}", "").split())
    return ""


def step_script(block, name):
    lines = block.splitlines()
    try:
        start = lines.index(f"      - name: {name}")
        run_line = lines.index("        run: |", start + 1)
    except ValueError as exc:
        raise AssertionError(f"missing literal run script for step {name!r}") from exc

    body = []
    for line in lines[run_line + 1 :]:
        if line and not line.startswith("          "):
            break
        body.append(line[10:] if line else line)
    return "\n".join(body)


class ReleaseWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.jobs = job_blocks(WORKFLOW_PATH.read_text(encoding="utf-8"))

    def test_job_scheduling_contract(self):
        needs = {
            "runtime-ready": ("setup", "static-checks", "godot-runtime-build", "runtime-assets-build"),
            "assemble": ("setup", "runtime-ready", "web-build", "macos-build", "windows-build", "linux-build"),
            "publish-runtime": ("setup", "assemble"),
            "publish-spx": ("setup", "assemble", "publish-runtime"),
            "publish-web-package": ("setup", "publish-spx"),
            "finalize-spx": ("setup", "publish-spx", "publish-web-package"),
        }
        conditions = {
            "runtime-ready": (
                "!cancelled()&&needs.setup.result=='success'&&needs.static-checks.result=='success'&&"
                "((needs.setup.outputs.runtime_state=='ready'&&needs.godot-runtime-build.result=='skipped'&&"
                "needs.runtime-assets-build.result=='skipped')||(needs.setup.outputs.runtime_state=='missing'&&"
                "needs.godot-runtime-build.result=='success'&&needs.runtime-assets-build.result=='success'))"
            ),
            "assemble": (
                "!cancelled()&&needs.setup.result=='success'&&needs.runtime-ready.result=='success'&&"
                "(needs.setup.outputs.run_web!='true'||needs.web-build.result=='success')&&"
                "(needs.setup.outputs.run_macos!='true'||needs.macos-build.result=='success')&&"
                "(needs.setup.outputs.run_windows!='true'||needs.windows-build.result=='success')&&"
                "(needs.setup.outputs.run_linux!='true'||needs.linux-build.result=='success')"
            ),
            "publish-runtime": "!cancelled()&&inputs.operation!='dry-run'&&needs.assemble.result=='success'",
            "publish-spx": "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-runtime.result=='success'",
            "publish-web-package": "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-spx.result=='success'",
            "finalize-spx": "!cancelled()&&inputs.operation=='publish-release'&&needs.publish-web-package.result=='success'",
        }
        for platform in ("web", "macos", "windows", "linux"):
            job = f"{platform}-build"
            needs[job] = ("setup", "runtime-ready")
            conditions[job] = (
                "!cancelled()&&needs.runtime-ready.result=='success'&&"
                f"needs.setup.outputs.run_{platform}=='true'"
            )

        for job, expected in conditions.items():
            with self.subTest(job=job):
                self.assertEqual(job_needs(self.jobs[job]), needs[job])
                self.assertEqual(job_condition(self.jobs[job]), expected)

    def test_release_gate_is_operation_aware_and_fail_closed(self):
        block = self.jobs["release-gate"]
        terminal_jobs = (
            "setup",
            "assemble",
            "publish-runtime",
            "publish-spx",
            "publish-web-package",
            "finalize-spx",
        )
        self.assertEqual(job_needs(block), terminal_jobs)
        self.assertEqual(job_condition(block), "!cancelled()")
        self.assertIn("          OPERATION: ${{ inputs.operation }}", block)
        self.assertIn("          NEEDS_JSON: ${{ toJSON(needs) }}", block)

        script = step_script(block, "Require the selected operation's terminal jobs")
        cases = (
            ("dry-run", {"setup": "success", "assemble": "success"}, True),
            ("publish-runtime", {"setup": "success", "assemble": "success", "publish-runtime": "success"}, True),
            ("publish-release", {job: "success" for job in terminal_jobs}, True),
            ("dry-run", {"setup": "success", "assemble": "skipped"}, False),
            ("publish-runtime", {"setup": "success", "assemble": "success", "publish-runtime": "skipped"}, False),
            ("publish-release", {"setup": "success", "assemble": "success", "publish-runtime": "success", "publish-spx": "success", "publish-web-package": "skipped"}, False),
            ("unknown", {}, False),
        )
        for operation, results, succeeds in cases:
            with self.subTest(operation=operation, results=results):
                needs_json = {
                    job: {"result": results.get(job, "skipped")}
                    for job in terminal_jobs
                }
                completed = subprocess.run(
                    ["bash", "-euo", "pipefail", "-c", script],
                    env={
                        **os.environ,
                        "OPERATION": operation,
                        "NEEDS_JSON": json.dumps(needs_json),
                    },
                    capture_output=True,
                    text=True,
                    check=False,
                )
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)


if __name__ == "__main__":
    unittest.main()
