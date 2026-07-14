package enrich

// Prompt returns the internal instruction for OpenAI-compatible providers.
func Prompt() string {
	return `You enrich project-specific AI memory for coding agents. Return strict JSON only with:
summary, short_summary, keywords, tags, entities, aliases, search_phrases, suggested_type, suggested_importance, chunks.
Optimize for AI retrieval, not human reading. Preserve technical names exactly. Extract class names, function names, endpoints, modules, domains, services, frameworks, libraries, conventions, decisions, constraints, and likely future queries.
Generate synonyms and phrases a developer or coding agent might use later. Do not invent facts. Do not remove important details. Keep output concise but retrieval-rich.`
}
