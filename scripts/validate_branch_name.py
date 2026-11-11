#!/usr/bin/env python3

import argparse
import re
import sys
from pathlib import Path

VALID_TYPES = {
    "feat",
    "fix",
    "docs",
    "style",
    "refactor",
    "test",
    "ci",
    "build",
}
JIRA_RE = re.compile(r"^[A-Z][A-Z0-9]+-[0-9]+$")
SHORT_NAME_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
EXPECTED_FORMAT = "<type>/<jira-key>/<short-name>"
VALID_EXAMPLE = "feat/TGTG-24/otp-request"


def read_value(args):
    if args.file:
        return Path(args.file).read_text(encoding="utf-8").strip()
    if args.message is not None:
        return args.message.strip()
    if not sys.stdin.isatty():
        return sys.stdin.read().strip()
    raise ValueError("No branch name provided. Pass it as an argument or via --file.")


def validate_branch_name(name):
    value = (name or "").strip()
    if not value:
        raise ValueError("Branch name is empty.")

    segments = value.split("/")
    if len(segments) != 3:
        raise ValueError(
            f"Branch name '{value}' must contain exactly three segments: {EXPECTED_FORMAT}."
        )

    branch_type, jira_key, short_name = segments
    if branch_type not in VALID_TYPES:
        allowed = ", ".join(sorted(VALID_TYPES))
        raise ValueError(
            f"Unsupported branch type '{branch_type}'. Allowed types: {allowed}. "
            f"Expected format: {EXPECTED_FORMAT}. Example: {VALID_EXAMPLE}"
        )

    if not JIRA_RE.fullmatch(jira_key):
        raise ValueError(
            f"Invalid Jira key '{jira_key}'. Expected pattern [A-Z][A-Z0-9]+-[0-9]+. "
            f"Example: {VALID_EXAMPLE}"
        )

    if not short_name or not SHORT_NAME_RE.fullmatch(short_name):
        raise ValueError(
            f"Invalid short name '{short_name}'. Use lowercase letters, numbers, and hyphens only. "
            f"Expected format: {EXPECTED_FORMAT}. Example: {VALID_EXAMPLE}"
        )

    return True


def main():
    parser = argparse.ArgumentParser(
        description="Validate a Git branch name against the required convention."
    )
    parser.add_argument("message", nargs="?", help="Branch name to validate")
    parser.add_argument("--file", help="Read the branch name from a file")
    args = parser.parse_args()

    try:
        name = read_value(args)
        validate_branch_name(name)
    except ValueError as exc:
        print(f"Branch validation failed: {exc}", file=sys.stderr)
        print(f"Expected format: {EXPECTED_FORMAT}", file=sys.stderr)
        print(f"Example: {VALID_EXAMPLE}", file=sys.stderr)
        return 1

    print(f"Branch name is valid: {name}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
