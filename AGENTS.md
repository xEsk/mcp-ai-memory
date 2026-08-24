# Agent Instructions

- Always follow the style of the project you are working in.
- Always document new methods, following the style and language of nearby comments.
- Always propose simple, effective solutions without unnecessary technical complication.
- Avoid reimplementing existing code; use project facilities whenever possible.
- If there are doubts, ask before proceeding.

## Project Knowledge Base Workflow

Before unfamiliar project work, read `docs/ai/README.md` and open only the guides relevant to the task.

Treat current source and explicit user decisions as authoritative. Update affected `docs/ai/` files when durable architecture, workflows, commands, boundaries, or critical rules change; do not mirror transient implementation details there. Keep the knowledge base in English.

## Project Memory MCP

- Canonical project key: `mcp-ai-memory`
- Always use this value as `project` in `mcp__project_memory` tool calls for this repository.
- If durable project knowledge is discovered, store or update it through the Project Memory MCP using this key.

<!-- project-ai-context:start -->
## Project AI Context

- Project Memory project: `mcp-ai-memory`
- Codebase Memory project: `mcp-ai-memory`
- Reuse these identities across every branch and worktree; never derive new ones from checkout paths, branches, tasks, or worktree names.
- Before unfamiliar work involving architecture, APIs, workflows, conventions, integrations, or prior investigations, query Project Memory narrowly and broaden only when needed.
- Maintain Project Memory when a task creates, changes, or invalidates durable project knowledge. Update the existing fact or remove it when obsolete; write nothing when no durable fact changed.
- Never store transient progress, speculation, generic advice, raw logs, secrets, credentials, or personal data.
- Reuse the canonical Codebase Memory index. Verify freshness or coverage when material and inspect source for reported gaps; do not create branch- or worktree-specific indexes.
- Read `docs/ai/README.md` first and open only task-relevant guides. Update affected guides when durable architecture, commands, workflows, boundaries, or critical rules change.
- Source code, configuration, schemas, tests, and explicit user decisions override stale memory or generated documentation; repair stale context when found.
- `$generate-operational-memory` is manual-only. Never invoke it automatically.
<!-- project-ai-context:end -->

## Code Navigation and Refactoring

- When applicable, prefer using goland-index MCP tools for code navigation and refactoring.
