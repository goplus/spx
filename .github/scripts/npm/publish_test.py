#!/usr/bin/env python3

import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest
import zipfile

from test_support import (SCRIPTS, WORKFLOWS, action_steps, load_workflow, named_step,
                          run_command, run_shell, step_script, write_executable)

WEB_PACKAGE_WORKFLOW_PATH = WORKFLOWS / "publish_npm.yml"
PUBLISH = SCRIPTS / "npm/publish.py"
DEV_VERSION = "4.0.1-0.20260814000000-aaaaaaaaaaaa"
NPM_FIXTURE = r'''
import json
import os
from pathlib import Path
import sys

args = sys.argv[1:]
with Path(os.environ["TRACE"]).open("a") as trace:
    trace.write(" ".join(args) + "\n")
if args[0] == os.environ.get("FAIL_OPERATION"):
    sys.exit(7)
if args[0] == "pack":
    print(os.environ.get("MOCK_PACK_JSON", '[{"filename":"spx.tgz","integrity":"sha512-local"}]'))
elif args[0] == "view" and args[2] == "dist.integrity":
    if os.environ.get("MOCK_EXISTS") != "true":
        sys.exit(1)
    print(os.environ.get("MOCK_INTEGRITY", "sha512-local"))
elif args[0] == "view" and args[2] == "dist-tags":
    if os.environ.get("FAIL_OPERATION") == "dist-tags":
        sys.exit(7)
    print(os.environ.get("MOCK_TAGS_JSON", "{}"))
elif args[0] != "publish":
    sys.exit(99)
'''


def run_publish(command, *, cwd, env):
    return run_command((sys.executable, str(PUBLISH), command), cwd=cwd, env=env)


def npm_workspace(root):
    fake_bin = root / "fake-bin"
    fake_bin.mkdir()
    write_executable(fake_bin / "npm", f"#!{sys.executable}\n" + NPM_FIXTURE)
    write_executable(fake_bin / "go", f"#!{sys.executable}\n" + r'''
import os
from pathlib import Path
import sys

with Path(os.environ["GO_TRACE"]).open("a") as trace:
    trace.write(" ".join(sys.argv[1:]) + "\n")
if os.environ.get("GO_EXIT"):
    sys.exit(int(os.environ["GO_EXIT"]))
print("v4.0.1-0.20260814000000-aaaaaaaaaaaa")
''')
    with zipfile.ZipFile(root / "spx_web.zip", "w") as archive:
        for name, content, mode in (("runtime.js", b"runtime script", 0o755),
                                    ("assets/runtime.wasm", b"wasm bytes", 0o644)):
            entry = zipfile.ZipInfo(name)
            entry.external_attr = (stat.S_IFREG | mode) << 16
            archive.writestr(entry, content)
    return {
        "PATH": f"{fake_bin}:{os.environ['PATH']}", "TRACE": str(root / "trace"),
        "RELEASE_VERSION": "", "GO_TRACE": str(root / "go-trace"),
    }


class NpmPublishTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.web_package_workflow = load_workflow(WEB_PACKAGE_WORKFLOW_PATH)
        cls.web_package_jobs = cls.web_package_workflow["jobs"]

    def test_source_and_publication_share_the_release_version_input(self):
        job = self.web_package_jobs["publish-npm"]
        checkout = action_steps(job, "actions/checkout")[0]
        for name, command in (("Verify checked-out publication source", "verify-source"),
                              ("Publish web package to npm", "publish")):
            step = named_step(job, name)
            self.assertEqual(step["run"], f"python3 .github/scripts/npm/publish.py {command}")
            self.assertLess(job["steps"].index(checkout), job["steps"].index(step))
            self.assertEqual(step["env"]["RELEASE_VERSION"], "${{ inputs.release_version }}")
        self.assertNotIn("version", {step.get("id") for step in job["steps"]})
        build, = action_steps(job, "./.github/actions/build-web-bundle")
        self.assertEqual(build["if"], "inputs.release_artifact == ''")
        self.assertLess(job["steps"].index(build),
                        job["steps"].index(named_step(job, "Publish web package to npm")))

    def test_web_package_is_reusable_only_and_rejects_release_tags(self):
        self.assertEqual(set(self.web_package_workflow["on"]), {"workflow_call"})
        self.assertEqual(self.web_package_workflow["permissions"], {"contents": "read", "id-token": "write"})
        block = self.web_package_jobs["publish-npm"]
        validate_target = step_script(block, "Validate publication target")
        for target, release_version, release_artifact, succeeds in (
            ("a" * 40, "", "", True),
            ("a" * 40, "4.0.0", "web-release", True),
            ("a" * 40, "4.0.0", "", False),
            ("a" * 40, "", "web-release", False),
            ("dev", "", "", False),
            ("refs/heads/dev", "", "", False),
            ("A" * 40, "", "", False),
        ):
            with self.subTest(target=target, release_version=release_version, release_artifact=release_artifact):
                completed = run_shell(validate_target, env={
                    "RELEASE_ARTIFACT": release_artifact, "RELEASE_VERSION": release_version, "TARGET": target,
                })
                self.assertEqual(completed.returncode == 0, succeeds, completed.stderr)

        with tempfile.TemporaryDirectory() as repository:
            for command in (
                ("git", "init", "--quiet"),
                ("git", "config", "user.name", "SPX release test"),
                ("git", "config", "user.email", "release-test@example.invalid"),
                ("git", "config", "commit.gpgSign", "false"),
                ("git", "config", "tag.gpgSign", "false"),
                ("git", "commit", "--allow-empty", "--quiet", "-m", "test"),
                ("git", "tag", "v4.0.0"),
            ):
                subprocess.run(command, cwd=repository, check=True)
            target = run_command(("git", "rev-parse", "HEAD"), cwd=repository, check=True).stdout.strip()
            tagged = run_publish("verify-source", cwd=repository, env={"RELEASE_VERSION": "", "TARGET": target})
            self.assertNotEqual(tagged.returncode, 0, tagged.stdout)
            self.assertIn("exact release tag v4.0.0", tagged.stderr)
            release = run_publish("verify-source", cwd=repository, env={"RELEASE_VERSION": "4.0.0", "TARGET": target})
            self.assertEqual(release.returncode, 0, release.stderr)
            wrong_sha = run_publish("verify-source", cwd=repository,
                                    env={"RELEASE_VERSION": "4.0.0", "TARGET": "b" * 40})
            self.assertNotEqual(wrong_sha.returncode, 0, wrong_sha.stdout)
            self.assertIn("instead of publication target", wrong_sha.stderr)
            subprocess.run(("git", "tag", "--delete", "v4.0.0"), cwd=repository, capture_output=True, check=True)
            untagged = run_publish("verify-source", cwd=repository, env={"RELEASE_VERSION": "", "TARGET": target})
            self.assertEqual(untagged.returncode, 0, untagged.stderr)

    def test_existing_package_requires_matching_integrity_and_dist_tag(self):
        for release, version, tag in (("", DEV_VERSION, "dev"),
                                     ("4.0.0", "4.0.0", "latest"), ("4.0.1-rc.1", "4.0.1-rc.1", "next")):
            for exists, integrity, remote_tag, error in (
                (True, "sha512-local", version, ""),
                (True, "sha512-local", "older", "not selected by dist-tag"),
                (True, "sha512-different", version, "does not match"),
                (False, "", "", ""),
            ):
                with self.subTest(tag=tag, exists=exists, integrity=integrity, remote_tag=remote_tag), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    env = npm_workspace(root)
                    completed = run_publish("publish", cwd=root, env={
                        **env, "RELEASE_VERSION": release,
                        "MOCK_EXISTS": str(exists).lower(), "MOCK_INTEGRITY": integrity,
                        "MOCK_TAGS_JSON": json.dumps({tag: remote_tag}),
                    })
                    self.assertEqual(completed.returncode == 0, not error, completed.stderr)
                    if error:
                        self.assertIn(error, completed.stderr)
                    commands = (root / "trace").read_text().splitlines()
                    self.assertEqual(commands.count(f"view @xgo-pkgs/spx@{version} dist.integrity"), 1)
                    self.assertEqual([command for command in commands if command.startswith("publish ")],
                                     [] if exists else [f"publish .npm/packages/spx.tgz --access public --tag {tag}"])

    def test_publication_normalizes_versions_without_go_for_releases(self):
        for release, version in (("", DEV_VERSION), ("4.0.0", "4.0.0"), ("v4.0.0", "4.0.0")):
            with self.subTest(release=release), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                env = npm_workspace(root)
                result = run_publish("publish", cwd=root, env={**env, "RELEASE_VERSION": release})
                self.assertEqual(result.returncode, 0, result.stderr)
                metadata = json.loads((root / ".npm/spx/package.json").read_text())
                self.assertEqual(metadata["version"], version)
                self.assertIn(f"SPX version: {version}", result.stdout)
                if release:
                    self.assertFalse((root / "go-trace").exists())
                else:
                    self.assertEqual((root / "go-trace").read_text(), "run ./.github/cmd/npm-version HEAD\n")

    def test_development_version_failure_stops_before_preparation_and_npm(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            env = npm_workspace(root)
            existing = root / ".npm/spx/keep.txt"
            existing.parent.mkdir(parents=True)
            existing.write_text("untouched package")
            # A missing bundle would fail preparation. The Go failure must win
            # before the package directory is cleaned or npm is invoked.
            (root / "spx_web.zip").unlink()
            result = run_publish("publish", cwd=root, env={**env, "GO_EXIT": "7"})
            self.assertEqual(result.returncode, 7, result.stderr)
            self.assertEqual(existing.read_text(), "untouched package")
            self.assertFalse((root / "trace").exists())
            self.assertFalse((root / ".npm/packages").exists())

    def test_npm_dist_tags_and_publication_failures(self):
        for release, dist_tag in (
            ("", "dev"),
            ("4.0.0", "latest"),
            ("4.0.1-rc.1", "next"),
        ):
            for failure in ("", "pack", "publish"):
                with self.subTest(dist_tag=dist_tag, failure=failure), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    env = npm_workspace(root)
                    result = run_publish("publish", cwd=root, env={
                        **env, "FAIL_OPERATION": failure, "RELEASE_VERSION": release,
                    })
                    self.assertEqual(result.returncode, 7 if failure else 0, result.stderr)
                    commands = (root / "trace").read_text().splitlines()
                    if failure == "pack":
                        self.assertEqual(len(commands), 1)
                    else:
                        self.assertEqual(commands[-1], f"publish .npm/packages/spx.tgz --access public --tag {dist_tag}")

    def test_package_preparation_preserves_payload_and_only_cleans_generated_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            env = npm_workspace(root)
            package = root / ".npm/spx"
            package.mkdir(parents=True)
            (package / "stale.js").write_text("old file")
            sibling = root / ".npm/packages/keep.tgz"
            sibling.parent.mkdir()
            sibling.write_bytes(b"previous package")
            result = run_publish("publish", cwd=root, env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((package / "stale.js").exists())
            self.assertEqual(sibling.read_bytes(), b"previous package")
            self.assertEqual((package / "runtime.js").read_bytes(), b"runtime script")
            self.assertEqual((package / "assets/runtime.wasm").read_bytes(), b"wasm bytes")
            self.assertEqual(stat.S_IMODE((package / "runtime.js").stat().st_mode), 0o755)
            metadata = json.loads((package / "package.json").read_text())
            self.assertEqual(metadata["name"], "@xgo-pkgs/spx")
            self.assertEqual(metadata["version"], DEV_VERSION)
            self.assertEqual(metadata["files"], ["**/*"])
            self.assertEqual(metadata["repository"]["url"], "git+https://github.com/goplus/spx.git")
            self.assertIn("generated from `spx_web.zip`", (package / "README.md").read_text())

    def test_invalid_bundle_never_calls_npm_or_writes_outside_package(self):
        for fixture in ("missing", "invalid", "empty", "parent path", "absolute path", "symlink"):
            with self.subTest(fixture=fixture), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                env = npm_workspace(root)
                bundle = root / "spx_web.zip"
                if fixture == "missing":
                    bundle.unlink()
                elif fixture == "invalid":
                    bundle.write_text("not a ZIP")
                elif fixture == "empty":
                    with zipfile.ZipFile(bundle, "w"):
                        pass
                else:
                    name = {"parent path": "../outside", "absolute path": "/outside", "symlink": "link"}[fixture]
                    with zipfile.ZipFile(bundle, "w") as archive:
                        member = zipfile.ZipInfo(name)
                        if fixture == "symlink":
                            member.external_attr = (stat.S_IFLNK | 0o777) << 16
                        archive.writestr(member, "outside")
                result = run_publish("publish", cwd=root, env=env)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertFalse((root / "trace").exists())
                self.assertFalse((root / "outside").exists())
                self.assertFalse((root / ".npm/outside").exists())

    def test_cleanup_does_not_follow_npm_directory_links(self):
        for linked in (".npm", ".npm/spx"):
            with self.subTest(linked=linked), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                env = npm_workspace(root)
                outside = root / "outside"
                (outside / "spx").mkdir(parents=True)
                protected = outside / "spx/keep"
                protected.write_text("untouched")
                link = root / linked
                link.parent.mkdir(parents=True, exist_ok=True)
                link.symlink_to(outside, target_is_directory=True)
                result = run_publish("publish", cwd=root, env=env)
                self.assertEqual(result.returncode == 0, linked == ".npm/spx", result.stderr)
                self.assertEqual(protected.read_text(), "untouched")
                if linked == ".npm":
                    self.assertFalse((root / "trace").exists())
                else:
                    self.assertFalse(link.is_symlink())

    def test_malformed_npm_responses_stop_before_publish(self):
        for response in ("not JSON", "[]", "{}", '[{}]',
                         '[{"filename":"../outside.tgz","integrity":"sha512-local"}]',
                         '[{"filename":"spx.tgz"}]'):
            with self.subTest(response=response), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                env = npm_workspace(root)
                result = run_publish("publish", cwd=root, env={**env, "MOCK_PACK_JSON": response})
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual((root / "trace").read_text().splitlines(),
                                 ["pack .npm/spx --json --pack-destination .npm/packages"])
        for response, failure in (("not JSON", ""), ("[]", ""), ("{}", ""), ("{}", "dist-tags")):
            with self.subTest(tags=response, failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                env = npm_workspace(root)
                result = run_publish("publish", cwd=root, env={
                    **env, "MOCK_EXISTS": "true", "MOCK_TAGS_JSON": response, "FAIL_OPERATION": failure,
                })
                self.assertEqual(result.returncode, 7 if failure else 1, result.stderr)
                self.assertFalse(any(command.startswith("publish ") for command in (root / "trace").read_text().splitlines()))


if __name__ == "__main__":
    unittest.main()
