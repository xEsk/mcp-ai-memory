package memory

import (
	"context"
	"fmt"
	"strings"

	"mcp-ai-memory/internal/enrich"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
)

// Service coordinates memory validation, enrichment, and persistence.
type Service struct {
	repo     *Repository
	projects *project.Service
	enricher enrich.Enricher
	fallback enrich.Enricher
	clock    platform.Clock
}

// NewService creates the memory service.
func NewService(repo *Repository, projects *project.Service, enricher enrich.Enricher, clock platform.Clock) *Service {
	return &Service{
		repo:     repo,
		projects: projects,
		enricher: enricher,
		fallback: enrich.NewLocalHeuristicEnricher(),
		clock:    clock,
	}
}

// Upsert creates or updates a memory while always preserving the original content.
func (s *Service) Upsert(ctx context.Context, input UpsertInput) (*Memory, error) {
	p, err := s.projects.MustGetByKey(ctx, input.Project)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.Content)
	if title == "" || content == "" {
		return nil, fmt.Errorf("%w: project, title and content are required", ErrInvalidInput)
	}
	memoryType, err := NormalizeType(input.Type)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid memory type %q", ErrInvalidInput, input.Type)
	}
	importance := input.Importance
	if importance < 1 {
		importance = 1
	}
	if importance > 5 {
		importance = 5
	}

	enriched := &enrich.EnrichOutput{SuggestedType: memoryType, SuggestedImportance: importance, Tags: input.Tags}
	if input.Enrich {
		enriched, err = s.enricher.EnrichMemory(ctx, enrich.EnrichInput{
			Project: input.Project, Type: memoryType, Title: title, Content: content, Tags: input.Tags, Importance: importance,
		})
		if err != nil {
			enriched, err = s.fallback.EnrichMemory(ctx, enrich.EnrichInput{
				Project: input.Project, Type: memoryType, Title: title, Content: content, Tags: input.Tags, Importance: importance,
			})
			if err != nil {
				return nil, fmt.Errorf("heuristic enrichment failed: %w", err)
			}
		}
	} else {
		enriched, err = s.fallback.EnrichMemory(ctx, enrich.EnrichInput{
			Project: input.Project, Type: memoryType, Title: title, Content: content, Tags: input.Tags, Importance: importance,
		})
		if err != nil {
			return nil, fmt.Errorf("heuristic enrichment failed: %w", err)
		}
	}

	if enriched.SuggestedType != "" && IsValidType(enriched.SuggestedType) && input.Type == "" {
		memoryType = enriched.SuggestedType
	}
	if enriched.SuggestedImportance >= 1 {
		importance = enriched.SuggestedImportance
		if importance > 5 {
			importance = 5
		}
	}

	now := s.clock.Now()
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = platform.NewID()
	}
	stored, err := s.repo.Upsert(ctx, Memory{
		ID:            id,
		ProjectID:     p.ID,
		ProjectKey:    p.Key,
		Type:          memoryType,
		Title:         title,
		Content:       content,
		Summary:       enriched.Summary,
		ShortSummary:  enriched.ShortSummary,
		Keywords:      enriched.Keywords,
		Tags:          mergeStrings(input.Tags, enriched.Tags),
		Entities:      enriched.Entities,
		Aliases:       enriched.Aliases,
		SearchPhrases: enriched.SearchPhrases,
		Importance:    importance,
		Source:        strings.TrimSpace(input.Source),
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		return nil, err
	}
	stored.ProjectKey = p.Key
	return stored, nil
}

// Get returns a memory after validating project ownership.
func (s *Service) Get(ctx context.Context, projectKey, id string) (*Memory, error) {
	p, err := s.projects.MustGetByKey(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	m, err := s.repo.GetByID(ctx, p.ID, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if err := s.repo.RecordAccesses(ctx, p.ID, []string{m.ID}, now); err != nil {
		return nil, err
	}
	m.AccessCount++
	m.LastAccessedAt = &now
	m.ProjectKey = p.Key
	return m, nil
}

// List returns active memories for a project.
func (s *Service) List(ctx context.Context, input ListInput) ([]Memory, error) {
	p, err := s.projects.MustGetByKey(ctx, input.Project)
	if err != nil {
		return nil, err
	}
	if input.Type != "" && !IsValidType(input.Type) {
		return nil, fmt.Errorf("%w: invalid memory type %q", ErrInvalidInput, input.Type)
	}
	items, err := s.repo.List(ctx, input, p.ID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	ids := make([]string, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
		items[i].AccessCount++
		items[i].LastAccessedAt = &now
		items[i].ProjectKey = p.Key
	}
	if err := s.repo.RecordAccesses(ctx, p.ID, ids, now); err != nil {
		return nil, err
	}
	return items, nil
}

// Delete marks a memory as deleted without removing physical data.
func (s *Service) Delete(ctx context.Context, projectKey, id string) error {
	p, err := s.projects.MustGetByKey(ctx, projectKey)
	if err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, p.ID, strings.TrimSpace(id), s.clock.Now())
}

func mergeStrings(left, right []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range [][]string{left, right} {
		for _, value := range group {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
