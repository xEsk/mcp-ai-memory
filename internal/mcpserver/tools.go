package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/search"
)

func (s *Server) registerTools() {
	s.mcp.AddTool(mcp.NewTool("memory.server.info",
		mcp.WithDescription("Return the memory MCP server name and version."),
	), s.traceTool("memory.server.info", s.handleServerInfo))

	s.mcp.AddTool(mcp.NewTool("memory.search",
		mcp.WithDescription("Search project-scoped AI memories with SQLite FTS5 over original and enriched fields."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project key")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Contextual search query")),
		mcp.WithString("mode", mcp.Enum("precise", "balanced", "broad"), mcp.Description("Search strictness, default precise")),
		mcp.WithNumber("limit", mcp.Description("Maximum results, default 10")),
		mcp.WithArray("types", mcp.WithStringItems()),
		mcp.WithArray("tags", mcp.WithStringItems()),
		mcp.WithBoolean("include_content", mcp.Description("Include full memory content")),
	), s.traceTool("memory.search", s.handleMemorySearch))

	s.mcp.AddTool(mcp.NewTool("memory.search.count",
		mcp.WithDescription("Count project-scoped memory search matches without returning memories or recording access."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project key")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Contextual search query")),
		mcp.WithString("mode", mcp.Enum("precise", "balanced", "broad"), mcp.Description("Search strictness, default precise")),
		mcp.WithArray("types", mcp.WithStringItems()),
		mcp.WithArray("tags", mcp.WithStringItems()),
	), s.traceTool("memory.search.count", s.handleMemorySearchCount))

	s.mcp.AddTool(mcp.NewTool("memory.search.debug",
		mcp.WithDescription("Debug project-scoped memory search ranking with snippets, scores, and match explanations."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project key")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Contextual search query")),
		mcp.WithString("mode", mcp.Enum("precise", "balanced", "broad"), mcp.Description("Search strictness, default precise")),
		mcp.WithNumber("limit", mcp.Description("Maximum results, default 10")),
		mcp.WithArray("types", mcp.WithStringItems()),
		mcp.WithArray("tags", mcp.WithStringItems()),
	), s.traceTool("memory.search.debug", s.handleMemorySearchDebug))

	s.mcp.AddTool(mcp.NewTool("memory.upsert",
		mcp.WithDescription("Create or update a memory in one project and refresh its search index."),
		mcp.WithString("project", mcp.Required()),
		mcp.WithString("id"),
		mcp.WithString("type", mcp.Enum("architecture", "decision", "api", "implementation", "bug", "workflow", "convention", "domain", "other")),
		mcp.WithString("title", mcp.Required()),
		mcp.WithString("content", mcp.Required()),
		mcp.WithArray("tags", mcp.WithStringItems()),
		mcp.WithNumber("importance"),
		mcp.WithString("source"),
		mcp.WithBoolean("enrich", mcp.Description("Generate retrieval metadata"), mcp.DefaultBool(true)),
	), s.traceTool("memory.upsert", s.handleMemoryUpsert))

	s.mcp.AddTool(mcp.NewTool("memory.delete",
		mcp.WithDescription("Soft-delete a project-scoped memory."),
		mcp.WithString("project", mcp.Required()),
		mcp.WithString("id", mcp.Required()),
		mcp.WithString("reason"),
	), s.traceTool("memory.delete", s.handleMemoryDelete))

	s.mcp.AddTool(mcp.NewTool("memory.get",
		mcp.WithDescription("Get one memory by ID inside a project."),
		mcp.WithString("project", mcp.Required()),
		mcp.WithString("id", mcp.Required()),
	), s.traceTool("memory.get", s.handleMemoryGet))

	s.mcp.AddTool(mcp.NewTool("memory.list",
		mcp.WithDescription("List active memories in a project."),
		mcp.WithString("project", mcp.Required()),
		mcp.WithString("type"),
		mcp.WithString("tag"),
		mcp.WithNumber("limit"),
		mcp.WithNumber("offset"),
	), s.traceTool("memory.list", s.handleMemoryList))

	s.mcp.AddTool(mcp.NewTool("project.upsert",
		mcp.WithDescription("Create or update a memory project."),
		mcp.WithString("key", mcp.Required()),
		mcp.WithString("name", mcp.Required()),
		mcp.WithString("description"),
	), s.traceTool("project.upsert", s.handleProjectUpsert))

	s.mcp.AddTool(mcp.NewTool("project.list",
		mcp.WithDescription("List memory projects."),
	), s.traceTool("project.list", s.handleProjectList))

	s.mcp.AddTool(mcp.NewTool("memory.suggest",
		mcp.WithDescription("Analyze new information and suggest whether to create, update, split, replace, or ignore memories."),
		mcp.WithString("project", mcp.Required()),
		mcp.WithString("input", mcp.Required()),
		mcp.WithString("context"),
	), s.traceTool("memory.suggest", s.handleMemorySuggest))
}

func (s *Server) handleServerInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = ctx
	_ = request
	return mcp.NewToolResultStructuredOnly(map[string]any{
		"name":    serverName,
		"version": serverVersion,
	}), nil
}

func (s *Server) handleMemorySearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memorySearchInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.search arguments: %v", err), nil
	}
	if strings.TrimSpace(input.Project) == "" || strings.TrimSpace(input.Query) == "" {
		return toolError("project and query are required"), nil
	}
	p, err := s.projects.MustGetByKey(ctx, input.Project)
	if err != nil {
		return toolError("%v", err), nil
	}
	results, err := s.search.Search(ctx, p.ID, p.Key, search.Input(input))
	if err != nil {
		if errors.Is(err, search.ErrInvalidMode) {
			return toolError("%v", err), nil
		}
		return nil, err
	}
	return mcp.NewToolResultStructuredOnly(map[string]any{"results": newSearchOutputs(results, input.IncludeContent)}), nil
}

func (s *Server) handleMemorySearchCount(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memorySearchCountInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.search.count arguments: %v", err), nil
	}
	if strings.TrimSpace(input.Project) == "" || strings.TrimSpace(input.Query) == "" {
		return toolError("project and query are required"), nil
	}
	p, err := s.projects.MustGetByKey(ctx, input.Project)
	if err != nil {
		return toolError("%v", err), nil
	}
	count, mode, err := s.search.Count(ctx, p.ID, p.Key, search.Input{
		Project: input.Project,
		Query:   input.Query,
		Mode:    input.Mode,
		Types:   input.Types,
		Tags:    input.Tags,
	})
	if err != nil {
		if errors.Is(err, search.ErrInvalidMode) {
			return toolError("%v", err), nil
		}
		return nil, err
	}
	return mcp.NewToolResultStructuredOnly(map[string]any{
		"project": input.Project,
		"query":   input.Query,
		"mode":    mode,
		"count":   count,
	}), nil
}

func (s *Server) handleMemorySearchDebug(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memorySearchDebugInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.search.debug arguments: %v", err), nil
	}
	if strings.TrimSpace(input.Project) == "" || strings.TrimSpace(input.Query) == "" {
		return toolError("project and query are required"), nil
	}
	p, err := s.projects.MustGetByKey(ctx, input.Project)
	if err != nil {
		return toolError("%v", err), nil
	}
	results, err := s.search.Search(ctx, p.ID, p.Key, search.Input{
		Project: input.Project,
		Query:   input.Query,
		Mode:    input.Mode,
		Limit:   input.Limit,
		Types:   input.Types,
		Tags:    input.Tags,
	})
	if err != nil {
		if errors.Is(err, search.ErrInvalidMode) {
			return toolError("%v", err), nil
		}
		return nil, err
	}
	return mcp.NewToolResultStructuredOnly(map[string]any{
		"project": input.Project,
		"query":   input.Query,
		"mode":    searchMode(input.Mode),
		"results": newSearchDebugOutputs(results),
	}), nil
}

func (s *Server) handleMemoryUpsert(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memoryUpsertInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.upsert arguments: %v", err), nil
	}
	if _, ok := request.GetArguments()["enrich"]; !ok {
		input.Enrich = true
	}
	stored, err := s.memories.Upsert(ctx, memory.UpsertInput(input))
	if err != nil {
		return toolError("%v", err), nil
	}
	return mcp.NewToolResultStructuredOnly(newMemoryOutput(*stored, true)), nil
}

func (s *Server) handleMemoryDelete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memoryDeleteInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.delete arguments: %v", err), nil
	}
	if err := s.memories.Delete(ctx, input.Project, input.ID); err != nil {
		return toolError("%v", err), nil
	}
	return mcp.NewToolResultStructuredOnly(map[string]any{"success": true, "id": input.ID}), nil
}

func (s *Server) handleMemoryGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memoryGetInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.get arguments: %v", err), nil
	}
	m, err := s.memories.Get(ctx, input.Project, input.ID)
	if err != nil {
		return toolError("%v", err), nil
	}
	return mcp.NewToolResultStructuredOnly(newMemoryOutput(*m, true)), nil
}

func (s *Server) handleMemoryList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memoryListInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.list arguments: %v", err), nil
	}
	items, err := s.memories.List(ctx, memory.ListInput(input))
	if err != nil {
		return toolError("%v", err), nil
	}
	return mcp.NewToolResultStructuredOnly(map[string]any{"memories": newMemoryOutputs(items, true)}), nil
}

func (s *Server) handleProjectUpsert(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input projectUpsertInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid project.upsert arguments: %v", err), nil
	}
	p, err := s.projects.Upsert(ctx, project.UpsertInput(input))
	if err != nil {
		return toolError("%v", err), nil
	}
	return mcp.NewToolResultStructuredOnly(p), nil
}

func (s *Server) handleProjectList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = request
	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultStructuredOnly(map[string]any{"projects": projects}), nil
}

func (s *Server) handleMemorySuggest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input memorySuggestInput
	if err := request.BindArguments(&input); err != nil {
		return toolError("invalid memory.suggest arguments: %v", err), nil
	}
	if strings.TrimSpace(input.Project) == "" || strings.TrimSpace(input.Input) == "" {
		return toolError("project and input are required"), nil
	}
	p, err := s.projects.MustGetByKey(ctx, input.Project)
	if err != nil {
		return toolError("%v", err), nil
	}
	query := input.Input + " " + input.Context
	mode := search.ModePrecise
	results, err := s.search.Search(ctx, p.ID, p.Key, search.Input{
		Project:        input.Project,
		Query:          query,
		Mode:           mode,
		Limit:          5,
		IncludeContent: false,
	})
	if err != nil {
		if errors.Is(err, search.ErrInvalidMode) {
			return toolError("%v", err), nil
		}
		return nil, err
	}
	if len(results) == 0 {
		mode = search.ModeBalanced
		results, err = s.search.Search(ctx, p.ID, p.Key, search.Input{
			Project:        input.Project,
			Query:          query,
			Mode:           mode,
			Limit:          5,
			IncludeContent: false,
		})
		if err != nil {
			if errors.Is(err, search.ErrInvalidMode) {
				return toolError("%v", err), nil
			}
			return nil, err
		}
	}
	candidates := evaluateSuggestCandidates(query, mode, results)
	recommendedCandidateID := recommendedSuggestCandidateID(candidates)
	action := "create"
	if recommendedCandidateID != "" {
		action = "update_existing"
	}
	if len(strings.Fields(input.Input)) < 5 {
		action = "ignore"
		recommendedCandidateID = ""
	}
	if strings.Count(input.Input, "\n\n") >= 2 {
		action = "split"
		recommendedCandidateID = ""
	}
	candidates = applySuggestActionToCandidates(candidates, action)
	return mcp.NewToolResultStructuredOnly(map[string]any{
		"project":                  input.Project,
		"suggested_action":         action,
		"reason":                   suggestionReason(action, mode, len(results)),
		"search_mode":              mode,
		"recommended_candidate_id": recommendedCandidateID,
		"confidence":               suggestionConfidence(action, mode, len(results)),
		"next_step":                suggestionNextStep(action),
		"candidate_count":          len(results),
		"candidates":               newSuggestCandidateOutputs(candidates),
		"modifies_memory":          false,
	}), nil
}

func toolError(format string, args ...any) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf(format, args...))
}

func suggestionReason(action, mode string, candidateCount int) string {
	switch action {
	case "update_existing":
		return "a strong existing memory candidate was found"
	case "split":
		return "input contains multiple separated sections and may be better as several memories"
	case "ignore":
		return "input is too short to be useful project memory"
	default:
		if candidateCount > 0 {
			return "similar candidates exist, but none are strong enough to recommend an update"
		}
		if mode == search.ModeBalanced {
			return "no close existing memory was found after precise and balanced search"
		}
		return "no close existing memory was found"
	}
}

func suggestionConfidence(action, mode string, candidateCount int) string {
	if candidateCount == 0 {
		return "none"
	}
	if action == "split" || action == "ignore" {
		return "low"
	}
	if action == "create" {
		return "low"
	}
	if mode == search.ModePrecise {
		return "high"
	}
	return "medium"
}

func suggestionNextStep(action string) string {
	switch action {
	case "update_existing":
		return "inspect_recommended_candidate"
	case "split":
		return "split_input"
	case "ignore":
		return "ignore_input"
	default:
		return "create_focused_memory"
	}
}

func searchMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return search.ModePrecise
	}
	return mode
}
