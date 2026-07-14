package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"mcp-ai-memory/internal/enrich"
	"mcp-ai-memory/internal/mcpserver"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/search"
	"mcp-ai-memory/internal/storage"
	"mcp-ai-memory/internal/trace"
)

// App groups the server's main dependencies.
type App struct {
	DB     *sql.DB
	Server *mcpserver.Server
}

// New builds the full application from configuration.
func New(ctx context.Context, cfg Config) (*App, error) {
	if err := validateTraceConfig(cfg); err != nil {
		return nil, err
	}
	db, err := storage.OpenSQLite(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	clock := platform.NewClock()
	projectService := project.NewService(project.NewRepository(db), clock)
	enricher := buildEnricher(cfg)
	memoryService := memory.NewService(memory.NewRepository(db), projectService, enricher, clock)
	searchService := search.NewService(search.NewFTS(db))
	tracer, err := buildTraceSink(cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	server := mcpserver.New(projectService, memoryService, searchService, tracer)

	return &App{DB: db, Server: server}, nil
}

// Close releases application resources.
func (a *App) Close() error {
	if a.DB == nil {
		return nil
	}
	if err := a.DB.Close(); err != nil {
		return fmt.Errorf("closing database: %w", err)
	}
	return nil
}

func buildEnricher(cfg Config) enrich.Enricher {
	if cfg.Enricher == "openai-compatible" || cfg.Enricher == "openai" {
		return enrich.NewOpenAICompatibleEnricher(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	}
	return enrich.NewLocalHeuristicEnricher()
}

func buildTraceSink(cfg Config) (trace.Sink, error) {
	if cfg.TraceMode == TraceModeOff {
		return trace.NoopSink{}, nil
	}
	if err := validateTraceConfig(cfg); err != nil {
		return nil, err
	}
	monitor, err := trace.NewMonitorSink(cfg.TraceMonitorURL, 25*time.Millisecond, cfg.TraceDebug)
	if err != nil {
		return nil, fmt.Errorf("build trace monitor sink: %w", err)
	}
	async := trace.NewAsyncSink(monitor, cfg.TraceBufferSize)
	sessionID := fmt.Sprintf("%d-%d", time.Now().UTC().UnixNano(), os.Getpid())
	var sink trace.Sink = async
	if cfg.TraceMode == TraceModeMetadata {
		sink = trace.NewMetadataSink(sink)
	}
	return trace.NewSessionSink(sessionID, os.Getpid(), sink), nil
}

// validateTraceConfig validates explicit trace consent before runtime resources are opened.
func validateTraceConfig(cfg Config) error {
	switch cfg.TraceMode {
	case TraceModeOff:
		return nil
	case TraceModeMetadata, TraceModeRaw:
		if err := trace.ValidateMonitorURL(cfg.TraceMonitorURL); err != nil {
			return fmt.Errorf("validate trace monitor URL: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("invalid trace mode %q: must be off, metadata, or raw", cfg.TraceMode)
	}
}
