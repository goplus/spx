#!/usr/bin/env python3
"""Build SPX's pen regression fixture and check actual macOS OpenGL pixels."""
import argparse
import json
import os
from pathlib import Path
import platform
import subprocess


def run(command, name, cwd, environment):
    with (output / name).open("w") as log:
        process = subprocess.Popen(command, cwd=cwd, env=environment,
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        for line in process.stdout:
            print(line, end="", flush=True)
            log.write(line)
        if process.wait():
            raise SystemExit(process.returncode)


fixture = Path(__file__).resolve().parent
repo = fixture.parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--skip-build", action="store_true")
parser.add_argument("--binary", type=Path, help="Existing binary containing the pen_validation module")
args = parser.parse_args()
godot = Path(os.environ.get("GODOT_SRC", str(repo / "godot"))).resolve()
module = repo / "godot_modules/spx"
output = repo / ".tmp/pen-validation"
output.mkdir(parents=True, exist_ok=True)
environment = dict(os.environ, SPX_PEN_SHADER_PATH=str(repo / "cmd/spx/template/project/engine/shader/spx_sprite_shader.gdshader"),
                   SPX_PEN_OUTPUT_DIR=str(output))
architecture = platform.machine()
binary = args.binary or godot / f"bin/godot.macos.editor.dev.{architecture}.pen_validation"

if not args.skip_build:
    subprocess.run(["make", "buildctl"], cwd=repo, env=environment, check=True)
    profile = json.loads((module / "spx_scons_profile.json").read_text())
    scons_version = json.loads((repo / "internal/release/runtime.lock.json").read_text())["toolchain"]["scons"]
    command = [str(repo / ".bin/buildctl"), "engine", "exec",
               "--lock-dir", str(godot / ".spx_build_lock"), "--workdir", str(godot), "--",
               str(repo / ".bin" / ("scons-" + scons_version) / "bin/scons"),
               *profile["common"], *profile["editor_release"],
               "platform=macos", "target=editor", "dev_build=yes", "tests=yes",
               "arch=" + architecture, "extra_suffix=pen_validation",
               "custom_modules=" + str(module) + "," + str(fixture / "pen_validation"),
               "-j" + str(min(os.cpu_count() or 1, 8))]
    run(command, "build.log", repo, environment)

run([str(binary), "--headless", "--test", "--test-case=*SPX*"], "unit.log", repo, environment)
run([str(binary), "--path", str(fixture / "project"), "--script", "validate.gd",
     "--display-driver", "macos", "--rendering-method", "gl_compatibility",
     "--rendering-driver", "opengl3", "--audio-driver", "Dummy",
     "--log-file", str(output / "godot.log"), "--quit-after", "300"], "gpu.log", repo, environment)
if "PEN_GPU_DONE" not in (output / "gpu.log").read_text():
    raise SystemExit("GPU checks did not reach their completion marker; see .tmp/pen-validation/gpu.log")
