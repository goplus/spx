"""Small test helpers shared by CI, release, and npm contract tests."""

import os
from pathlib import Path
import subprocess

import yaml

SCRIPTS = Path(__file__).resolve().parents[1]
REPO_ROOT = SCRIPTS.parents[1]
WORKFLOWS = SCRIPTS.parent / "workflows"


def load_workflow(path):
    # BaseLoader preserves GitHub's `on` key and string-valued action inputs.
    return yaml.load(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def run_command(command, *, cwd=None, env=None, check=False):
    return subprocess.run(command, cwd=cwd, env={**os.environ, **(env or {})},
                          capture_output=True, text=True, check=check)


def run_shell(script, *, cwd=None, env=None):
    return run_command(["bash", "-euo", "pipefail", "-c", script], cwd=cwd, env=env)


def write_executable(path, content):
    path.write_text(content, encoding="utf-8")
    path.chmod(0o755)


def job_needs(job):
    needs = job.get("needs", [])
    return tuple(needs if isinstance(needs, list) else [needs])


def job_condition(job):
    return "".join(job.get("if", "").replace("${{", "").replace("}}", "").split())


def named_step(job, name):
    return next(step for step in job["steps"] if step.get("name") == name)


def step_script(job, name):
    return named_step(job, name)["run"]


def action_steps(job, action):
    return [step for step in job["steps"] if step.get("uses", "").split("@")[0] == action]
