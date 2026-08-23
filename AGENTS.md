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

## Code Navigation and Refactoring

- When applicable, prefer using goland-index MCP tools for code navigation and refactoring.
