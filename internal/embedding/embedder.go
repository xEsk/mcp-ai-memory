package embedding

import "context"

// Embedder converts text into vectors for future hybrid search.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// VectorResult represents a vector match.
type VectorResult struct {
	MemoryID string
	Score    float64
}

// VectorSearch abstracts the future vector store.
type VectorSearch interface {
	UpsertEmbedding(ctx context.Context, memoryID string, projectID string, embedding []float32) error
	Search(ctx context.Context, projectID string, embedding []float32, limit int) ([]VectorResult, error)
}
