# Contributing

This project uses **spec-driven development**: every feature or significant change starts
with a written specification before any code is written. This keeps intent and implementation
aligned and makes PRs easier to review.

The workflow is powered by [Spec Kit](https://github.com/github-spec-kit/spec-kit) slash
commands, available in Claude Code.

---

## The workflow

```
/speckit-specify  →  /speckit-plan  →  /speckit-tasks  →  /speckit-implement
```

### 1. Write the spec — `/speckit-specify <description>`

Describe the feature in plain English. Spec Kit creates a spec file under `specs/NNN-your-feature/`
and asks a few clarifying questions if needed.

```
/speckit-specify Add support for connecting to Slack as an upstream MCP server
```

The spec captures: user stories, functional requirements, success criteria, and edge cases —
in plain language, with no implementation details.

### 2. Plan the implementation — `/speckit-plan`

Analyzes the spec and produces `plan.md` (tech stack choices, architecture, file structure)
and `contracts/` (API shapes, data model). This is where technology decisions are made.

```
/speckit-plan
```

Review `plan.md` before moving on. This is the right moment to push back on approach.

### 3. Generate tasks — `/speckit-tasks`

Breaks the plan into a concrete, ordered task list in `tasks.md`. Tasks are small enough to
complete and verify individually, with parallelism noted where possible.

```
/speckit-tasks
```

### 4. Implement — `/speckit-implement`

Works through `tasks.md` in order, checks off each task as it completes, and stops if
anything blocks progress.

```
/speckit-implement
```

---

## Before opening a PR

Run the branch validator to confirm your branch name and spec directory are wired up:

```
/speckit-git-validate
```

Branch names follow the pattern `NNN-short-description` matching the spec directory,
e.g. `002-slack-upstream`.

Make sure lint and tests are clean:

```bash
make lint
go test ./...
```

---

## For small changes

Not everything needs a full spec. Use your judgement:

| Change | Use speckit? |
|--------|-------------|
| New upstream server type | Yes |
| New user-facing feature | Yes |
| Significant refactor | Yes |
| Bug fix | No — PR directly |
| README / docs update | No — PR directly |
| Dependency bump | No — PR directly |
| CI / tooling change | No — PR directly |

---

## Useful make targets

```bash
make run          # build and start the server locally (starts Postgres via Docker)
make test         # run unit tests
make lint         # check for lint issues
make lint-fix     # auto-fix lint issues
make vuln         # run govulncheck for known vulnerabilities
make db-reset     # drop and recreate the local database
```
