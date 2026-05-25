# Implementation Plan: AI Tool Auto-Configuration

**Branch**: `009-ai-tool-autoconfig` | **Date**: 2026-05-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/007-ai-tool-autoconfig/spec.md`

## Summary

Add a dashboard section that detects installed AI tools (Claude Desktop, Gemini CLI), shows their MCP configuration status, and lets the user write the proxy endpoint into each tool's config in one click. Claude Desktop config is written via atomic JSON merge; Gemini CLI is configured via `gemini mcp add` subprocess. Local mode only.

## Technical Context

**Language/Version**: Go 1.23 (existing project)

**Primary Dependencies**: Standard library only — `encoding/json`, `os`, `os/exec`; no new third-party packages required.

**Storage**: No new storage. The proxy's own base URL (scheme + host + port) is the only dynamic value written to tool configs.

**Testing**: `go test ./...` (existing); new handler and aitools package unit tests.

**Target Platform**: macOS (local mode only); gated by `cfg.LocalMode`.

**Project Type**: Web service — new HTTP handler routes added to the existing `internal/handler` package.

**Performance Goals**: Detection and status checks must complete in under 500ms on page load; `gemini mcp list` subprocess timeout 5s.

**Constraints**: Config writes must be atomic (temp-file rename on macOS). Feature is local-mode-only (`cfg.LocalMode` gate). No new dependencies beyond stdlib.

**Scale/Scope**: Two tools at launch (Claude Desktop, Gemini CLI). Additive per-tool structs make future tools easy to add.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Simplicity First | PASS | stdlib only; no new abstractions beyond a small `aitools` package |
| II. Dual-Deployment | PASS | Feature gated by `cfg.LocalMode`; no-op in hosted mode |
| III. MCP Protocol Fidelity | PASS | No MCP session changes |
| IV. Security by Design | PASS | Reads/writes only local config files the user already owns |

No violations. Complexity Tracking table not required.

## Project Structure

### Documentation (this feature)

```text
specs/007-ai-tool-autoconfig/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── api.md
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code

```text
internal/
├── aitools/             # NEW — tool detection, config read/write
│   ├── registry.go      # AITool interface + registry of supported tools
│   ├── claude_desktop.go  # Claude Desktop detection + config writer
│   └── gemini_cli.go    # Gemini CLI detection + config writer
└── handler/
    └── aitools.go       # NEW — HTTP handlers for GET /api/tools, POST /api/tools/{id}/configure

templates/
└── partials/
    └── ai-tools.html    # NEW — dashboard partial for the AI Tools section

static/
└── js/
    └── ai-tools.js      # NEW — fetch-based configure button logic
```

**Structure Decision**: New `internal/aitools` package owns all tool-specific logic (detection, status, config mutation). The handler package adds a thin HTTP layer on top. The dashboard partial is wired into the existing dashboard template. This keeps tool-specific code isolated and testable without pulling in HTTP concerns.
