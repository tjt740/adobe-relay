#!/usr/bin/env python3
"""Add the dedicated Clash secret without changing existing deployment credentials."""
import os
from pathlib import Path
import secrets
import sys


def ensure_secret(destination: Path) -> bool:
    # This is an upgrade of an existing deployment, never an account reset.
    original = destination.read_text()
    for line in original.splitlines():
        key, separator, value = line.partition("=")
        if separator and key.strip() == "CLASH_CONTROLLER_SECRET":
            if not value.strip().strip("\"'"):
                raise ValueError("CLASH_CONTROLLER_SECRET exists but is empty")
            return False
    with destination.open("a") as output:
        if original and not original.endswith("\n"):
            output.write("\n")
        output.write("CLASH_CONTROLLER_SECRET=" + secrets.token_hex(32) + "\n")
    os.chmod(destination, 0o600)
    return True


if __name__ == "__main__":
    changed = ensure_secret(Path(sys.argv[1]))
    print("Added Clash controller secret." if changed else "Existing Clash secret preserved.")
