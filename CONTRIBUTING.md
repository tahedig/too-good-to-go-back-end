# Contributing

This repository uses a strict Git workflow to keep pull requests reviewable and to ensure every change is traceable to a Jira issue. All branch names and commit messages must follow the conventions below.

## Branch naming convention

All development branches must follow the format:

```text
<type>/<jira-key>/<short-name>
```

Allowed branch types:

- `feat`
- `fix`
- `docs`
- `style`
- `refactor`
- `test`
- `ci`
- `build`

Examples:

```text
feat/TGTG-24/otp-request
fix/TGTG-35/login-validation
docs/TGTG-42/update-api-documentation
refactor/TGTG-50/user-service
test/TGTG-55/add-user-tests
ci/TGTG-60/github-actions
build/TGTG-65/update-dependencies
style/TGTG-70/format-handlers
```

### Component meanings

- `type`: the category of change.
- `jira-key`: the associated Jira issue key, using the pattern `[A-Z][A-Z0-9]+-[0-9]+`.
- `short-name`: a short, descriptive name in lowercase, using hyphen-separated words.

### Valid and invalid examples

Valid:

```text
feat/TGTG-24/otp-request
fix/TGTG-35/login-validation
ci/TGTG-60/github-actions
```

Invalid:

```text
feature/otp-request
Feat/TGTG-24/otp-request
feat/tgtg-24/otp-request
feat/TGTG-24
feat/TGTG-24/OTP_Request
```

The common issues are:

- use of an unsupported type such as `feature` instead of `feat`
- lowercase Jira keys such as `tgtg-24`
- missing the short-name segment
- uppercase letters or underscores in the short name

To fix an invalid branch name, rename it before opening a pull request:

```bash
git branch -m feature/otp-request
```

A correct replacement is:

```bash
git branch -m feat/TGTG-24/otp-request
```

## Commit message convention

Every commit must follow this exact format:

```text
<type>: <short summary> issue: <Jira issue key> description: <brief explanation of the change>
```

Allowed commit types:

- `feat`
- `fix`
- `docs`
- `style`
- `refactor`
- `test`
- `ci`
- `build`

### Purpose of each type

| Type | Purpose |
|---|---|
| `feat` | Introduces a new feature |
| `fix` | Fixes a bug |
| `docs` | Changes documentation |
| `style` | Changes formatting or code style without changing behavior |
| `refactor` | Restructures code without changing behavior |
| `test` | Adds or updates tests |
| `ci` | Changes CI/CD configuration |
| `build` | Changes build configuration or dependencies |

### Required fields

All three required fields must appear in order:

1. `type: summary`
2. `issue: <jira-key>`
3. `description: <meaningful explanation>`

Examples:

```text
feat: add OTP request endpoint issue: TGTG-24 description: Add phone validation and create an OTP through the configured SMS provider.
fix: correct login validation issue: TGTG-35 description: Reject empty or malformed phone values before attempting authentication.
docs: update API documentation issue: TGTG-42 description: Clarify the OTP request and verification endpoints for client developers.
style: format route handlers issue: TGTG-70 description: Align indentation and spacing without altering handler behavior.
refactor: simplify user service issue: TGTG-50 description: Extract validation logic to a shared helper to reduce duplication.
test: add auth router tests issue: TGTG-55 description: Cover invalid and successful OTP flows in the router test suite.
ci: add PR validation workflow issue: TGTG-60 description: Enforce branch naming, commit conventions, and quality checks for every pull request.
build: update Go dependencies issue: TGTG-65 description: Refresh the toolchain and pinned dependency versions used by the service.
```

### Invalid commit examples

```text
added OTP endpoint

feature: add OTP endpoint issue: TGTG-24 description: Implement OTP.

feat: add OTP endpoint description: Implement OTP.

feat: add OTP endpoint issue: description: Implement OTP.

feat: issue: TGTG-24 description: Implement OTP.

feat: add OTP endpoint issue: tgtg-24 description: Implement OTP.

feat: add OTP endpoint issue: TGTG-24 description:
```

Why these are invalid:

- The first example is missing the required `type:` prefix.
- `feature` is not an allowed type.
- The `issue:` field is missing entirely.
- The `issue:` field is present but empty.
- The summary is empty.
- The Jira key must be uppercase and follow the `TEAM-123` pattern.
- The description is empty.

### Correcting a commit message

If the last commit is wrong, amend it locally before pushing:

```bash
git commit --amend -m "feat: add OTP request endpoint issue: TGTG-24 description: Add phone validation and create an OTP through the configured SMS provider."
```

If several commits need to be rewritten, use an interactive rebase:

```bash
git rebase -i HEAD~N
```

Then change the action from `pick` to `reword` for the commits that need updates, and save. After rewriting commits, force-push with lease:

```bash
git push --force-with-lease
```

## Development workflow

### 1. Create a branch

```bash
git switch -c feat/TGTG-24/otp-request
```

### 2. Check the current branch

```bash
git branch --show-current
```

### 3. Stage changes

```bash
git add .
```

### 4. Create a compliant commit

```bash
git commit -m "feat: add OTP request endpoint issue: TGTG-24 description: Add phone validation and create an OTP through the configured SMS provider."
```

### 5. Push the branch

```bash
git push -u origin feat/TGTG-24/otp-request
```

### 6. Open a pull request

Open the pull request in GitHub and ensure the branch and commit conventions are satisfied before requesting review.

### 7. Review CI results

The pull request checks verify:

- branch naming
- commit message discipline
- Go formatting
- Go linting
- Go tests
- Go build

### 8. Fix validation failures

If a check fails, fix the offending files and recommit using the same rules. For example:

```bash
git add .
git commit -m "fix: correct PR validation script issue: TGTG-24 description: Ensure the branch validator accepts valid Jira keys and rejects invalid names."
```

### 9. Update the pull request

After corrections, push the branch again:

```bash
git push
```

The pull request will automatically refresh with the latest validation result.

## Repository automation

The repository includes automated GitHub Actions checks to enforce the same rules on every pull request, including all commits in the PR branch. These checks are intended to complement the existing GitHub ruleset approval requirement without replacing it.
