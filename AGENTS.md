# Agent Instructions

- Always follow the style of the project you are working in.
- Always document new methods, following the style and language of nearby comments.
- Always propose simple, effective solutions without unnecessary technical complication.
- Avoid reimplementing existing code; use project facilities whenever possible.
- If there are doubts, ask before proceeding.

## Project Knowledge Base Workflow

Future AI agents must use the persistent project knowledge base before broad repository analysis.

1. Read `docs/ai/README.md` first.
2. Understand the basic project context and identify the task type.
3. Open only the `docs/ai/` files relevant to the current task.
4. Do not analyze the entire repository immediately.
5. Respect existing project conventions and preserve project continuity.
6. Do not redesign architecture unless explicitly requested.
7. If code contradicts the knowledge base, trust code and update the knowledge base afterward.
8. If a new subsystem, workflow, or important decision is added, update `docs/ai/`.
9. Keep the knowledge base in English.

## Project Memory MCP

- Canonical project key: `mcp-ai-memory`
- Always use this value as `project` in `mcp__project_memory` tool calls for this repository.
- If durable project knowledge is discovered, store or update it through the Project Memory MCP using this key.

## Code Navigation and Refactoring

- When applicable, prefer using goland-index MCP tools for code navigation and refactoring.
