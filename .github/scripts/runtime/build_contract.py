#!/usr/bin/env python3
"""Validate and expose the runtime build contract to GitHub Actions.

The runtime lock is consumed before the pinned Go toolchain is available, so
this small, dependency-free Python CLI is shared by the SPX and Godot jobs.
It writes only explicitly requested GitHub step outputs; source checkout and
build side effects remain in the calling actions.
"""

import argparse
import hashlib
import json
import re
import sys
import unicodedata
from pathlib import Path, PurePosixPath
from urllib.parse import urlsplit


RUNTIME_VERSION_PATTERN = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$")
GIT_COMMIT_PATTERN = re.compile(r"^[0-9a-f]{40}$")
REPOSITORY_COMPONENT_PATTERN = re.compile(r"^[A-Za-z0-9_.-]+$")
RELEASE_REPOSITORY_PATTERN = re.compile(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$")
CANONICAL_GITHUB_REPOSITORY_PATTERN = re.compile(
    r"^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+\.git$"
)
TOOLCHAIN_VERSION_PATTERN = re.compile(r"^[0-9A-Za-z][0-9A-Za-z.+_-]*$")
FLAG_KEY_PATTERN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")

LOCK_FIELDS = {
    "schema",
    "runtime_version",
    "runtime_abi",
    "release_repository",
    "manifest",
    "required_assets",
    "godot",
    "module",
    "toolchain",
}
GODOT_FIELDS = {"repository", "ref", "commit", "version"}
MODULE_FIELDS = {"path"}
TOOLCHAIN_FIELDS = ("go", "xgo", "scons", "emsdk", "android_ndk", "jdk")
ENGINE_TOOLCHAIN_SCOPES = {
    "native": ("spx-engine-toolchain-native-v1", ("scons",)),
    "web": ("spx-engine-toolchain-web-v1", ("scons", "emsdk")),
    "android": ("spx-engine-toolchain-android-v1", ("scons", "jdk", "android_ndk")),
}
PROFILE_FIELDS = {"schema", "common", "editor_release", "template_release"}
PROFILE_GROUPS = ("common", "editor_release", "template_release")
ORCHESTRATION_KEYS = {
    "angle",
    "arch",
    "custom_modules",
    "dev_build",
    "generate_bundle",
    "ios_simulator",
    "linker",
    "platform",
    "proxy_to_pthread",
    "target",
    "tests",
    "threads",
    "use_llvm",
    "vsproj",
}

# setup-ndk accepts release aliases rather than package revisions. Keep this
# intentionally explicit: unknown revisions are tolerated by non-Android jobs
# and rejected only when --require-android-ndk-alias is requested.
ANDROID_NDK_INSTALLER_ALIASES = {
    "23.2.8568313": "r23c",
}


class ContractError(ValueError):
    """A user-facing runtime build contract validation error."""


def reject_duplicate_fields(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ContractError(f"duplicate JSON field: {key!r}")
        result[key] = value
    return result


def reject_json_constant(value):
    raise ContractError(f"invalid JSON constant: {value}")


def load_json(path, label):
    try:
        return json.loads(
            Path(path).read_text(encoding="utf-8"),
            object_pairs_hook=reject_duplicate_fields,
            parse_constant=reject_json_constant,
        )
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ContractError(f"cannot read {label}: {error}") from error


def require_exact_fields(value, expected, label):
    if type(value) is not dict:
        raise ContractError(f"{label} must be an object")
    actual = set(value)
    if actual != set(expected):
        missing = sorted(set(expected) - actual)
        extra = sorted(actual - set(expected))
        raise ContractError(f"{label} fields mismatch; missing={missing}, extra={extra}")


def validate_basename(value, label):
    if (
        type(value) is not str
        or not value
        or value != value.strip()
        or value in (".", "..")
        or "/" in value
        or "\\" in value
        or any(unicodedata.category(character) == "Cc" for character in value)
    ):
        raise ContractError(f"{label} must be a non-empty basename")
    return value


def validate_module_path(value):
    if type(value) is not str or not value:
        raise ContractError("module.path must be a non-empty string")
    if "\\" in value or any(unicodedata.category(character) == "Cc" for character in value):
        raise ContractError("module.path must use a single-line POSIX path")
    module_path = PurePosixPath(value)
    if value == "." or not module_path.parts or module_path.is_absolute() or value != module_path.as_posix():
        raise ContractError("module.path must be a normalized relative path")
    if any(part in ("", ".", "..") for part in module_path.parts):
        raise ContractError("module.path must not contain empty, current, or parent segments")
    if any(REPOSITORY_COMPONENT_PATTERN.fullmatch(part) is None for part in module_path.parts):
        raise ContractError("module.path contains an invalid path segment")
    return module_path.as_posix()


def normalize_github_repository(value, label):
    if type(value) is not str or not value:
        raise ContractError(f"{label} must be a non-empty string")
    if value != value.strip() or any(character in value for character in "\0\r\n\t"):
        raise ContractError(f"{label} must be a single GitHub repository URL")

    scp_match = re.fullmatch(r"git@github\.com:(.+)", value, flags=re.IGNORECASE)
    if scp_match:
        repository_path = scp_match.group(1)
    elif re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:\.git)?/?", value):
        repository_path = value
    else:
        url_value = "https://" + value if value.lower().startswith("github.com/") else value
        try:
            parsed = urlsplit(url_value)
            port = parsed.port
        except ValueError as error:
            raise ContractError(f"{label} is not a valid GitHub repository URL: {error}") from error
        allowed_ports = {
            "http": {None, 80},
            "https": {None, 443},
            "ssh": {None, 22},
            "git+ssh": {None, 22},
            "git": {None, 9418},
        }
        scheme = parsed.scheme.lower()
        if scheme not in allowed_ports or port not in allowed_ports[scheme]:
            raise ContractError(f"{label} must use a standard GitHub HTTP, SSH, or Git URL")
        if (parsed.hostname or "").lower() != "github.com":
            raise ContractError(f"{label} must refer to github.com")
        if parsed.query or parsed.fragment:
            raise ContractError(f"{label} must not contain a query or fragment")
        if scheme in ("ssh", "git+ssh") and parsed.username not in (None, "git"):
            raise ContractError(f"{label} SSH URLs must use the git user")
        repository_path = parsed.path.lstrip("/")

    repository_path = repository_path.rstrip("/")
    if repository_path.lower().endswith(".git"):
        repository_path = repository_path[:-4]
    parts = repository_path.split("/")
    if (
        len(parts) != 2
        or any(part in ("", ".", "..") for part in parts)
        or any(REPOSITORY_COMPONENT_PATTERN.fullmatch(part) is None for part in parts)
    ):
        raise ContractError(f"{label} must identify exactly one GitHub owner/repository")
    return "/".join(parts).lower()


def load_runtime_lock(path):
    return validate_runtime_lock(load_json(path, "runtime lock"))


def validate_runtime_lock(lock):
    require_exact_fields(lock, LOCK_FIELDS, "runtime lock")
    if type(lock["schema"]) is not int or lock["schema"] != 1:
        raise ContractError("runtime lock schema must be the integer 1")
    if type(lock["runtime_version"]) is not str or RUNTIME_VERSION_PATTERN.fullmatch(lock["runtime_version"]) is None:
        raise ContractError("runtime_version must be a semantic runtime version")
    if type(lock["runtime_abi"]) is not int or lock["runtime_abi"] < 1:
        raise ContractError("runtime_abi must be a positive integer")
    if type(lock["release_repository"]) is not str or RELEASE_REPOSITORY_PATTERN.fullmatch(lock["release_repository"]) is None:
        raise ContractError("release_repository must use owner/repository form")
    manifest = validate_basename(lock["manifest"], "manifest")

    assets = lock["required_assets"]
    if type(assets) is not list or not assets:
        raise ContractError("required_assets must be a non-empty array")
    seen_assets = set()
    for index, asset in enumerate(assets):
        validate_basename(asset, f"required_assets[{index}]")
        if asset == manifest:
            raise ContractError(f"required asset {asset!r} conflicts with manifest")
        if asset in seen_assets:
            raise ContractError(f"duplicate required asset {asset!r}")
        seen_assets.add(asset)
    if assets != sorted(assets):
        raise ContractError("required_assets must be sorted by name")

    godot = lock["godot"]
    require_exact_fields(godot, GODOT_FIELDS, "godot")
    if (
        type(godot["repository"]) is not str
        or CANONICAL_GITHUB_REPOSITORY_PATTERN.fullmatch(godot["repository"]) is None
    ):
        raise ContractError("godot.repository must use canonical URL https://github.com/owner/repository.git")
    if type(godot["ref"]) is not str or not godot["ref"] or any(
        character.isspace() or unicodedata.category(character) == "Cc" for character in godot["ref"]
    ):
        raise ContractError("godot.ref must be non-empty without whitespace or control characters")
    if type(godot["commit"]) is not str or GIT_COMMIT_PATTERN.fullmatch(godot["commit"]) is None:
        raise ContractError("godot.commit must be a 40-character lowercase SHA-1")
    if type(godot["version"]) is not str or not godot["version"] or any(
        character.isspace() or unicodedata.category(character) == "Cc" for character in godot["version"]
    ):
        raise ContractError("godot.version must be non-empty without whitespace or control characters")

    module = lock["module"]
    require_exact_fields(module, MODULE_FIELDS, "module")
    validate_module_path(module["path"])

    toolchain = lock["toolchain"]
    require_exact_fields(toolchain, TOOLCHAIN_FIELDS, "toolchain")
    for field in TOOLCHAIN_FIELDS:
        version = toolchain[field]
        if type(version) is not str or TOOLCHAIN_VERSION_PATTERN.fullmatch(version) is None:
            raise ContractError(f"toolchain.{field} must be a non-empty, single-token version")
    if re.fullmatch(r"[1-9][0-9]*", toolchain["jdk"]) is None:
        raise ContractError("toolchain.jdk must be a positive major version")
    return lock


def resolve_toolchain(lock, require_android_ndk_alias=False, toolchain_scope=None):
    toolchain = lock["toolchain"]
    ndk_version = toolchain["android_ndk"]
    ndk_alias = ANDROID_NDK_INSTALLER_ALIASES.get(ndk_version, "")
    if require_android_ndk_alias and not ndk_alias:
        raise ContractError(f"no validated setup-ndk alias for toolchain.android_ndk {ndk_version!r}")

    outputs = {
        "go-version": toolchain["go"],
        "xgo-version": toolchain["xgo"],
        "scons-version": toolchain["scons"],
        "emsdk-version": toolchain["emsdk"],
        "android-ndk-version": ndk_version,
        "android-ndk-installer-alias": ndk_alias,
        "jdk-version": toolchain["jdk"],
    }
    if toolchain_scope is not None:
        domain, fields = ENGINE_TOOLCHAIN_SCOPES[toolchain_scope]
        payload = domain + "\n" + "".join(f"{field}={toolchain[field]}\n" for field in fields)
        outputs["engine-toolchain-sha256"] = hashlib.sha256(payload.encode("utf-8")).hexdigest()
    return outputs


def describe_lock(lock):
    outputs = {
        "runtime-version": lock["runtime_version"],
        "runtime-abi": str(lock["runtime_abi"]),
        "release-repository": lock["release_repository"],
        "runtime-manifest": lock["manifest"],
        "godot-repository": normalize_github_repository(lock["godot"]["repository"], "godot.repository"),
        "godot-ref": lock["godot"]["ref"],
        "godot-commit": lock["godot"]["commit"],
        "godot-version": lock["godot"]["version"],
        "module-relative-path": lock["module"]["path"],
    }
    outputs.update(resolve_toolchain(lock))
    return outputs


def validate_source(lock, runtime_version, runtime_abi, godot_origin, godot_commit):
    requested_version = runtime_version[1:] if runtime_version.startswith("v") else runtime_version
    if RUNTIME_VERSION_PATTERN.fullmatch(requested_version) is None:
        raise ContractError("runtime version input must be semantic, optionally prefixed by v")
    if requested_version != lock["runtime_version"]:
        raise ContractError(
            f"runtime version input {requested_version!r} does not match runtime_version {lock['runtime_version']!r}"
        )
    if re.fullmatch(r"[1-9][0-9]*", runtime_abi) is None:
        raise ContractError("runtime ABI input must be a canonical positive integer")
    if int(runtime_abi) != lock["runtime_abi"]:
        raise ContractError(f"runtime ABI input {runtime_abi!r} does not match runtime_abi {lock['runtime_abi']!r}")

    locked_repository = normalize_github_repository(lock["godot"]["repository"], "godot.repository")
    current_repository = normalize_github_repository(godot_origin, "current Godot origin")
    if current_repository != locked_repository:
        raise ContractError(
            f"current Godot origin identifies {current_repository!r}; godot.repository identifies {locked_repository!r}"
        )
    if GIT_COMMIT_PATTERN.fullmatch(godot_commit) is None:
        raise ContractError("current Godot commit must be a 40-character lowercase SHA-1")
    if godot_commit != lock["godot"]["commit"]:
        raise ContractError(f"current Godot commit {godot_commit!r} does not match godot.commit {lock['godot']['commit']!r}")
    return lock["module"]["path"]


def load_scons_profile(path):
    profile = load_json(path, "SPX SCons profile")
    require_exact_fields(profile, PROFILE_FIELDS, "SPX SCons profile")
    if type(profile["schema"]) is not int or profile["schema"] != 1:
        raise ContractError("SPX SCons profile schema must be the integer 1")

    groups = {}
    group_keys = {}
    for group_name in PROFILE_GROUPS:
        tokens = profile[group_name]
        if type(tokens) is not list:
            raise ContractError(f"{group_name} must be an array")
        keys = set()
        for index, token in enumerate(tokens):
            if type(token) is not str or not token or any(
                character.isspace() or unicodedata.category(character) == "Cc" for character in token
            ):
                raise ContractError(f"{group_name}[{index}] must be a non-empty token without whitespace")
            if token.count("=") != 1:
                raise ContractError(f"{group_name}[{index}] must be a single key=value token")
            key, value = token.split("=", 1)
            if FLAG_KEY_PATTERN.fullmatch(key) is None or not value:
                raise ContractError(f"{group_name}[{index}] must have a valid non-empty key and value")
            if key in ORCHESTRATION_KEYS:
                raise ContractError(f"{group_name}[{index}] uses orchestration-owned key {key!r}")
            if key in keys:
                raise ContractError(f"{group_name} contains duplicate key {key!r}")
            keys.add(key)
        groups[group_name] = tokens
        group_keys[group_name] = keys

    for variant in ("editor_release", "template_release"):
        duplicates = sorted(group_keys["common"] & group_keys[variant])
        if duplicates:
            raise ContractError(f"common and {variant} contain duplicate keys: {duplicates}")
    return {
        "common-flags": " ".join(groups["common"]),
        "editor-release-flags": " ".join(groups["common"] + groups["editor_release"]),
        "template-release-flags": " ".join(groups["common"] + groups["template_release"]),
    }


def write_github_outputs(path, outputs):
    with Path(path).open("a", encoding="utf-8") as output_file:
        for name, value in outputs.items():
            if "\n" in value or "\r" in value:
                raise ContractError(f"GitHub output {name!r} must be a single line")
            print(f"{name}={value}", file=output_file)


def run_toolchain(args):
    lock = load_runtime_lock(args.lock)
    outputs = resolve_toolchain(lock, args.require_android_ndk_alias)
    write_github_outputs(args.github_output, outputs)


def run_describe(args):
    write_github_outputs(args.github_output, describe_lock(load_runtime_lock(args.lock)))


def run_source(args):
    lock = load_runtime_lock(args.lock)
    module_path = validate_source(
        lock,
        args.runtime_version,
        args.runtime_abi,
        args.godot_origin,
        args.godot_commit,
    )
    outputs = resolve_toolchain(lock, args.require_android_ndk_alias, args.toolchain_scope)
    outputs["module-relative-path"] = module_path
    write_github_outputs(args.github_output, outputs)
    print(module_path)


def run_profile(args):
    write_github_outputs(args.github_output, load_scons_profile(args.profile))


def add_output_arguments(parser, include_lock=False, include_ndk=False):
    if include_lock:
        parser.add_argument("--lock", required=True, help="path to runtime.lock.json")
    parser.add_argument("--github-output", required=True, help="path supplied by GitHub Actions in GITHUB_OUTPUT")
    if include_ndk:
        parser.add_argument(
            "--require-android-ndk-alias",
            action="store_true",
            help="fail unless the locked Android NDK has a validated setup-ndk alias",
        )


def build_parser():
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    toolchain = subparsers.add_parser("toolchain", help="emit locked toolchain versions")
    add_output_arguments(toolchain, include_lock=True, include_ndk=True)
    toolchain.set_defaults(handler=run_toolchain)

    describe = subparsers.add_parser("describe", help="emit immutable checkout and toolchain inputs from the lock")
    add_output_arguments(describe, include_lock=True)
    describe.set_defaults(handler=run_describe)

    source = subparsers.add_parser("source", help="validate Godot source identity and emit lock-derived inputs")
    add_output_arguments(source, include_lock=True, include_ndk=True)
    source.add_argument("--runtime-version", required=True)
    source.add_argument("--runtime-abi", required=True)
    source.add_argument("--godot-origin", required=True)
    source.add_argument("--godot-commit", required=True)
    source.add_argument("--toolchain-scope", required=True, choices=tuple(ENGINE_TOOLCHAIN_SCOPES))
    source.set_defaults(handler=run_source)

    profile = subparsers.add_parser("profile", help="validate and emit SPX SCons profile flags")
    profile.add_argument("--profile", required=True, help="path to spx_scons_profile.json")
    add_output_arguments(profile)
    profile.set_defaults(handler=run_profile)
    return parser


def main(argv=None):
    args = build_parser().parse_args(argv)
    try:
        args.handler(args)
    except (ContractError, OSError) as error:
        print(f"[error] Invalid runtime build contract: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
