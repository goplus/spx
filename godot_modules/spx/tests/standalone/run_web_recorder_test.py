#!/usr/bin/env python3
"""Run the real C++ Web recorder's C ABI lifecycle without a browser or Godot build."""

import os
from pathlib import Path
import shlex
import subprocess
import tempfile


HERE = Path(__file__).resolve().parent
RECORDER = HERE.parent.parent / "recorder"
HEADERS = (
    "core/config/project_settings.h",
    "core/error/error_list.h",
    "core/error/error_macros.h",
    "core/io/image.h",
    "core/math/vector2i.h",
    "core/os/os.h",
    "core/os/time.h",
    "core/string/ustring.h",
    "core/templates/list.h",
    "modules/modules_enabled.gen.h",
    "servers/audio_server.h",
    "servers/movie_writer/movie_writer.h",
    "emscripten.h",
)


def main():
    with tempfile.TemporaryDirectory(prefix="spx-web-recorder-") as directory:
        build = Path(directory)
        for name in HEADERS:
            header = build / name
            header.parent.mkdir(parents=True, exist_ok=True)
            header.write_text('#include "web_recorder_stubs.h"\n')
        executable = build / "web_recorder_test"
        subprocess.run([
            *shlex.split(os.environ.get("CXX", "c++")), "-std=c++17",
            "-Wall", "-Wextra", "-Werror", "-Wno-unused-parameter",
            "-DWEB_ENABLED", "-DMODULE_SPX_ENABLED",
            "-I", str(build), "-I", str(HERE), "-I", str(RECORDER),
            str(RECORDER / "movie_writer_webm.cpp"), str(HERE / "web_recorder_test.cpp"),
            "-o", str(executable),
        ], check=True)
        subprocess.run([str(executable)], check=True)


if __name__ == "__main__":
    main()
