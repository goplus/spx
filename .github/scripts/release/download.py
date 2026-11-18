"""Download release assets with bounded retries after interrupted transfers."""

from subprocess import CalledProcessError, run
import sys
import time


def download_release(tag, repository, directory):
    command = ("gh", "release", "download", tag, "--repo", repository,
               "--dir", str(directory), "--clobber")
    for attempt in range(1, 4):
        try:
            run(command, check=True)
            return
        except CalledProcessError as error:
            if attempt == 3 or error.returncode < 0:
                raise
            delay = 5 * 2 ** (attempt - 1)
            print(f"[warning] Download of {repository}@{tag} failed "
                  f"(attempt {attempt}/3, exit {error.returncode}); retrying in {delay}s", file=sys.stderr)
            time.sleep(delay)
