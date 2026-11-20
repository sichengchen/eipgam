#!/usr/bin/env python3
"""Choose the version to build from the stable tags in the source checkout."""

import re
import subprocess
import sys

SEMVER = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)")
FIRST_VERSION = (0, 2, 0)


def release_version(tags: list[str], bump: str) -> str:
    versions = [FIRST_VERSION]
    for tag in tags:
        match = SEMVER.fullmatch(tag)
        if match:
            versions.append(tuple(int(part) for part in match.groups()))
    major, minor, patch = max(versions)
    if bump == "patch":
        patch += 1
    elif bump == "minor":
        minor, patch = minor + 1, 0
    elif bump == "major":
        major, minor, patch = major + 1, 0, 0
    elif bump != "current":
        raise ValueError(f"Unknown release bump {bump!r}")
    return f"{major}.{minor}.{patch}"


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: release-version.py current|patch|minor|major")
    tags = subprocess.check_output(
        ["git", "tag", "--merged", "HEAD", "--list", "v*"], text=True
    ).splitlines()
    try:
        print(release_version(tags, sys.argv[1]))
    except ValueError as error:
        raise SystemExit(str(error))
