package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatibleEnricher calls a /chat/completions-compatible endpoint.
type OpenAICompatibleEnricher struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAICompatibleEnricher creates a configurable remote enricher.
func NewOpenAICompatibleEnricher(baseURL, apiKey, model string) *OpenAICompatibleEnricher {
	return &OpenAICompatibleEnricher{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// EnrichMemory requests strict JSON from the remote provider.
func (e *OpenAICompatibleEnricher) EnrichMemory(ctx context.Context, input EnrichInput) (*EnrichOutput, error) {
	body := map[string]any{
		"model": e.model,
		"messages": []map[string]string{
			{"role": "system", "content": Prompt()},
			{"role": "user", "content": fmt.Sprintf("Project: %s\nType: %s\nTitle: %s\nTags: %s\nContent:\n%s", input.Project, input.Type, input.Title, strings.Join(input.Tags, ", "), input.Content)},
		},
		"temperature": 0.1,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling enrichment request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating enrichment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling enrichment provider: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("enrichment provider returned status %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decoding enrichment response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("enrichment provider returned no choices")
	}
	var out EnrichOutput
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("decoding enrichment json: %w", err)
	}
	return &out, nil
}
