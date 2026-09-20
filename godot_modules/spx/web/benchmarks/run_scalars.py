#!/usr/bin/env python3
"""Build actual old/new Web wrappers with the same sinks, then measure in Node."""
import argparse
from pathlib import Path
import re
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
WEB = Path("godot_modules/spx/web")
FUNCTIONS = ["physics_set_global_gravity", "camera_set_camera_smoothing",
             "sprite_set_rotation", "sprite_set_visible", "ui_set_range"]


def source(revision, relative):
    if revision == "worktree":
        return (ROOT / relative).read_text()
    return subprocess.check_output(["git", "show", f"{revision}:{relative}"], cwd=ROOT, text=True)


def build(directory, label, revision):
    util = source(revision, WEB / "godot_js_spx_util.cpp")
    for include in ["../gdextension_spx_ext.h", "godot_js_spx_util.h"]:
        util = util.replace(f'"{include}"', f'"{(ROOT / WEB / include).resolve()}"')
    generated = source(revision, WEB / "godot_js_spx.cpp")
    wrappers = []
    for function in FUNCTIONS:
        match = re.search(r"void gdspx_" + function + r"\([^\n]*\) \{.*?\n\}", generated, re.S)
        if not match:
            raise ValueError(f"Missing generated function {function} in {revision}")
        wrappers.append("EMSCRIPTEN_KEEPALIVE\n" + match.group())
    cpp = directory / f"{label}.cpp"
    cpp.write_text((HERE / "scalar_bridge.cpp").read_text() + "\n" + util +
                   '\nextern "C" {\n' + "\n".join(wrappers) + "\n}\n")
    subprocess.run(["em++", str(cpp), "-std=c++17", "-O3", "-Wl,--wrap=malloc",
                    "-idirafter", str(ROOT / WEB / "tests/stubs"),
                    "-sMODULARIZE=1", "-sENVIRONMENT=node", "-sALLOW_MEMORY_GROWTH=1",
                    "-sASSERTIONS=0", "-sWASM_BIGINT=1", "-o", str(directory / f"{label}.cjs")], check=True)
    for kind, filename in [("util", "gdspx.util.js"), ("calls", "gdspx.js")]:
        (directory / f"{label}.{kind}.js").write_text(source(revision, WEB / "js/engine" / filename))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", default="f263460539", help="Git revision before scalar optimization")
    parser.add_argument("--candidate", default="worktree")
    parser.add_argument("--node", default="node")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="spx-web-scalars-") as temporary:
        directory = Path(temporary)
        build(directory, "before", args.baseline)
        build(directory, "after", args.candidate)
        subprocess.run([args.node, str(HERE / "scalars.cjs"), str(directory), args.baseline, args.candidate], check=True)


if __name__ == "__main__":
    main()
