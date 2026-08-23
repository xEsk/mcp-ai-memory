package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	TraceModeOff      = "off"
	TraceModeMetadata = "metadata"
	TraceModeRaw      = "raw"
)

// Config contains the server configuration loaded from environment variables.
type Config struct {
	DBPath            string
	Enricher          string
	LLMBaseURL        string
	LLMAPIKey         string
	LLMModel          string
	EmbeddingsEnabled bool
	EmbeddingBaseURL  string
	EmbeddingModel    string
	TraceMode         string
	TraceMonitorURL   string
	TraceBufferSize   int
	TraceDebug        bool
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() Config {
	return Config{
		DBPath:            os.Getenv("MEMORY_DB_PATH"),
		Enricher:          envString("MEMORY_ENRICHER", "heuristic"),
		LLMBaseURL:        envString("MEMORY_LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:         os.Getenv("MEMORY_LLM_API_KEY"),
		LLMModel:          envString("MEMORY_LLM_MODEL", "gpt-4.1-mini"),
		EmbeddingsEnabled: envBool("MEMORY_EMBEDDINGS_ENABLED", false),
		EmbeddingBaseURL:  envString("MEMORY_EMBEDDING_BASE_URL", "http://localhost:11434"),
		EmbeddingModel:    envString("MEMORY_EMBEDDING_MODEL", "nomic-embed-text"),
		TraceMode:         envString("MEMORY_TRACE_MODE", TraceModeOff),
		TraceMonitorURL:   envString("MEMORY_TRACE_MONITOR_URL", "http://127.0.0.1:7379/events"),
		TraceBufferSize:   envInt("MEMORY_TRACE_BUFFER_SIZE", 256),
		TraceDebug:        envBool("MEMORY_TRACE_DEBUG", false),
	}
}

// validateDBPath validates the configured database identity before storage access.
func validateDBPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("MEMORY_DB_PATH is required")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("MEMORY_DB_PATH must be absolute")
	}
	return nil
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
