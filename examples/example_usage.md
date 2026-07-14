# Example Usage

Client config examples:

- `claude_desktop_config.json` for Claude Desktop.
- `opencode.json` for OpenCode.

1. Create a project with `project.upsert`.
2. Store a memory with `memory.upsert`.
3. Ask contextual questions with `memory.search`.

Example search input:

```json
{
  "project": "example-api",
  "query": "how are API routes wired?",
  "limit": 10,
  "include_content": true
}
```
