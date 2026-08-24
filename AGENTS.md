# Agent Instructions

<!-- project-ai-context:start -->
## Project AI Context

- Project Memory project: `mcp-ai-memory`
- Codebase Memory project: `mcp-ai-memory`
- Reuse these identities across every branch and worktree; never derive new ones from checkout paths, branches, tasks, or worktree names.
- Before unfamiliar work involving architecture, APIs, workflows, conventions, integrations, or prior investigations, query Project Memory narrowly and broaden only when needed.
- Maintain Project Memory when a task creates, changes, or invalidates durable project knowledge. Update the existing fact or remove it when obsolete; write nothing when no durable fact changed.
- Never store transient progress, speculation, generic advice, raw logs, secrets, credentials, or personal data.
- Reuse the canonical Codebase Memory index. Verify freshness or coverage when material and inspect source for reported gaps; do not create branch- or worktree-specific indexes.
- Read `docs/ai/README.md` first and open only task-relevant guides. Keep the knowledge base in English and update affected guides when durable architecture, commands, workflows, boundaries, or critical rules change.
- Source code, configuration, schemas, tests, and explicit user decisions override stale memory or generated documentation; repair stale context when found.
- `$generate-operational-memory` is manual-only. Never invoke it automatically.
<!-- project-ai-context:end -->

## Code Navigation and Refactoring

- When applicable, prefer using goland-index MCP tools for code navigation and refactoring.
