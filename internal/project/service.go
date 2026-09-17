package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mcp-ai-memory/internal/platform"
)

var ErrProjectNotFound = errors.New("project not found")

// Service coordinates project operations.
type Service struct {
	repo  *Repository
	clock platform.Clock
}

// NewService creates a project service.
func NewService(repo *Repository, clock platform.Clock) *Service {
	return &Service{repo: repo, clock: clock}
}

// Upsert creates or updates a project using its key as the stable identifier.
func (s *Service) Upsert(ctx context.Context, input UpsertInput) (*Project, error) {
	key := strings.TrimSpace(input.Key)
	name := strings.TrimSpace(input.Name)
	if key == "" || name == "" {
		return nil, fmt.Errorf("project key and name are required")
	}
	now := s.clock.Now()
	return s.repo.Upsert(ctx, Project{
		ID:          platform.NewID(),
		Key:         key,
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

// MustGetByKey returns the project or a domain error when it does not exist.
func (s *Service) MustGetByKey(ctx context.Context, key string) (*Project, error) {
	p, err := s.repo.GetByKey(ctx, strings.TrimSpace(key))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return p, nil
}

// List returns known projects.
func (s *Service) List(ctx context.Context) ([]Project, error) {
	return s.repo.List(ctx)
}

// ListAll returns active and deleted projects for offline management.
func (s *Service) ListAll(ctx context.Context) ([]Project, error) {
	return s.repo.ListAll(ctx)
}

// Stats returns aggregate data for an active or deleted project.
func (s *Service) Stats(ctx context.Context, key string) (Stats, error) {
	p, err := s.repo.GetAnyByKey(ctx, strings.TrimSpace(key))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Stats{}, ErrProjectNotFound
		}
		return Stats{}, err
	}
	return s.repo.Stats(ctx, p.ID)
}

// SoftDelete hides a project and preserves its data for restoration.
func (s *Service) SoftDelete(ctx context.Context, key string) error {
	if err := s.repo.SoftDelete(ctx, strings.TrimSpace(key), s.clock.Now()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProjectNotFound
		}
		return err
	}
	return nil
}

// Restore makes a soft-deleted project active again.
func (s *Service) Restore(ctx context.Context, key string) error {
	if err := s.repo.Restore(ctx, strings.TrimSpace(key), s.clock.Now()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProjectNotFound
		}
		return err
	}
	return nil
}

// PreviewCleanup reports what permanent cleanup would remove.
func (s *Service) PreviewCleanup(ctx context.Context) (CleanupStats, error) {
	return s.repo.PreviewCleanup(ctx)
}

// Cleanup permanently removes previously deleted data and stale derived rows.
func (s *Service) Cleanup(ctx context.Context) (CleanupStats, error) {
	return s.repo.Cleanup(ctx)
}
