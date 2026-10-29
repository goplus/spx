#!/usr/bin/env python3
"""Run automation tests in separate processes with one shared import environment."""

import os
from pathlib import Path
import subprocess
import sys


def main():
    root = Path(__file__).resolve().parents[3]
    scripts = root / ".github/scripts"
    tests = [Path(path).resolve() for path in sys.argv[1:]] if len(sys.argv) > 1 else [
        *sorted(scripts.rglob("*_test.py")), root / "cmd/ispx/build_test.py",
    ]
    env = {**os.environ, "PYTHONDONTWRITEBYTECODE": "1",
           "PYTHONPATH": os.pathsep.join(map(str, (scripts, scripts / "ci")))}
    for test in tests:
        result = subprocess.run([sys.executable, str(test)], cwd=root, env=env)
        if result.returncode:
            return result.returncode
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
