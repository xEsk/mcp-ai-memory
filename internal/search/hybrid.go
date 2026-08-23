package search

import (
	"context"
	"encoding/json"
	"time"
)

// Service coordinates text search and leaves the extension point for vectors.
type Service struct {
	fts *FTS
}

// NewService creates the search service.
func NewService(fts *FTS) *Service {
	return &Service{fts: fts}
}

// Search always searches inside the requested project.
func (s *Service) Search(ctx context.Context, projectID, projectKey string, input Input) ([]Result, error) {
	return s.fts.Search(ctx, projectID, projectKey, input)
}

// SearchWithoutRecording searches without changing retrieval history.
func (s *Service) SearchWithoutRecording(ctx context.Context, projectID, projectKey string, input Input) ([]Result, error) {
	return s.fts.SearchWithoutRecording(ctx, projectID, projectKey, input)
}

// RecordAccesses records one retrieval for each distinct result memory.
func (s *Service) RecordAccesses(ctx context.Context, projectID string, results []Result) error {
	return s.fts.RecordAccesses(ctx, projectID, results)
}

// Count counts search results without recording accesses.
func (s *Service) Count(ctx context.Context, projectID, projectKey string, input Input) (int, string, error) {
	return s.fts.Count(ctx, projectID, projectKey, input)
}

func jsonList(value string) []string {
	var out []string
	_ = json.Unmarshal([]byte(value), &out)
	return out
}

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}
