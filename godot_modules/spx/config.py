def can_build(env, platform):
    return True


def get_opts(platform):
    from SCons.Variables import BoolVariable

    return [BoolVariable("spx_spine", "Enable the separately licensed Spine runtime", False)]


def configure(env):
    pass
