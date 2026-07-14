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
	db, err := storage.OpenSQLite(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	clock := platform.NewClock()
	projectService := project.NewService(project.NewRepository(db), clock)
	enricher := buildEnricher(cfg)
	memoryService := memory.NewService(memory.NewRepository(db), projectService, enricher, clock)
	searchService := search.NewService(search.NewFTS(db))
	tracer := buildTraceSink(cfg)
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

func buildTraceSink(cfg Config) trace.Sink {
	monitor := trace.NewMonitorSink(cfg.TraceMonitorURL, 25*time.Millisecond, cfg.TraceDebug)
	async := trace.NewAsyncSink(monitor, cfg.TraceBufferSize)
	sessionID := fmt.Sprintf("%d-%d", time.Now().UTC().UnixNano(), os.Getpid())
	return trace.NewSessionSink(sessionID, os.Getpid(), async)
}
