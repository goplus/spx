def can_build(env, platform):
    return True


def get_opts(platform):
    from SCons.Variables import BoolVariable

    return [BoolVariable("web_link_optimize", "Optimize WebAssembly and JavaScript during Web linking", True)]


def configure(env):
    if env["platform"] == "web" and not env["web_link_optimize"]:
        # Override Godot's earlier link flags without changing C/C++ compilation.
        # -O1 skips Binaryen optimization and JS/Wasm minification.
        env.Append(LINKFLAGS=["-O1"])
        print("SPX: Web link optimization disabled (-O1); set web_link_optimize=yes for release artifacts.")
