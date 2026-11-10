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
FORMAT = "<type>: <short summary> issue: <Jira issue key> description: <brief explanation>"
EXAMPLE = "feat: add OTP request endpoint issue: TGTG-24 description: Add phone validation and create an OTP through the configured SMS provider."


def read_subject(message_value=None, file_path=None):
    if file_path:
        content = Path(file_path).read_text(encoding="utf-8")
        for line in content.splitlines():
            if line.strip():
                return line.strip()
        raise ValueError("Commit message file is empty.")

    if message_value is not None:
        content = message_value
    else:
        content = sys.stdin.read()

    for line in content.splitlines():
        if line.strip():
            return line.strip()
    raise ValueError("Commit message is empty.")


def validate_commit_message(message):
    subject = (message or "").strip()
    if not subject:
        raise ValueError("Commit message is empty.")

    if subject.startswith("Merge "):
        raise ValueError("Merge commits are not valid for this repository convention.")

    pattern = (
        r"^(?P<type>feat|fix|docs|style|refactor|test|ci|build):\s+"
        r"(?P<summary>.+?)\s+"
        r"issue:\s+(?P<jira>[A-Z][A-Z0-9]+-[0-9]+)\s+"
        r"description:\s+(?P<description>.+)$"
    )
    match = re.fullmatch(pattern, subject)
    if not match:
        raise ValueError(
            "Commit message must follow the required format: "
            f"{FORMAT}. Example: {EXAMPLE}"
        )

    if not match.group("summary").strip():
        raise ValueError("Summary is required before 'issue:'.")
    if not match.group("description").strip():
        raise ValueError("Description is required after 'description:'.")
    if not JIRA_RE.fullmatch(match.group("jira")):
        raise ValueError(
            f"Invalid Jira key '{match.group('jira')}'. Expected pattern [A-Z][A-Z0-9]+-[0-9]+."
        )

    return True


def main():
    parser = argparse.ArgumentParser(
        description="Validate a commit message against the team's Git convention."
    )
    parser.add_argument("message", nargs="?", help="Commit subject to validate")
    parser.add_argument("--message", dest="message_flag", help="Commit subject to validate")
    parser.add_argument("--file", help="Read the commit message from a file")
    args = parser.parse_args()

    # Support both current CI usage (`--message`) and the old positional style.
    message_value = args.message_flag if args.message_flag is not None else args.message

    try:
        subject = read_subject(message_value=message_value, file_path=args.file)
        validate_commit_message(subject)
    except ValueError as exc:
        print(f"Commit validation failed: {exc}", file=sys.stderr)
        print(f"Required format: {FORMAT}", file=sys.stderr)
        print(f"Example: {EXAMPLE}", file=sys.stderr)
        return 1

    print(f"Commit message is valid: {subject}")
    return 0


if __name__ == "__main__":
    sys.exit(main())