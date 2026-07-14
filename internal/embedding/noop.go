package embedding

import "context"

// NoopEmbedder disables embeddings without changing the service architecture.
type NoopEmbedder struct{}

// Embed returns nil to indicate that no vector is available.
func (NoopEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	_ = text
	return nil, nil
}
