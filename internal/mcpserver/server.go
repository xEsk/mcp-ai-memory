package mcpserver

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/search"
	"mcp-ai-memory/internal/trace"
)

const (
	serverName    = "mcp-project-memory"
	serverVersion = "0.1.0"
)

// Server exposes memory services as MCP tools.
type Server struct {
	projects *project.Service
	memories *memory.Service
	search   *search.Service
	tracer   trace.Sink
	mcp      *server.MCPServer
}

// New registers all MCP tools supported by v1.
func New(projects *project.Service, memories *memory.Service, searchService *search.Service, tracer ...trace.Sink) *Server {
	traceSink := trace.Sink(trace.NoopSink{})
	if len(tracer) > 0 && tracer[0] != nil {
		traceSink = tracer[0]
	}
	s := &Server{
		projects: projects,
		memories: memories,
		search:   searchService,
		tracer:   traceSink,
		mcp: server.NewMCPServer(
			serverName,
			serverVersion,
			server.WithToolCapabilities(true),
		),
	}
	s.registerTools()
	return s
}

// ServeStdio starts the MCP stdio transport.
func (s *Server) ServeStdio(ctx context.Context) error {
	_ = ctx
	return server.ServeStdio(s.mcp)
}

type toolHandler = server.ToolHandlerFunc

func (s *Server) traceTool(name string, handler toolHandler) toolHandler {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		s.tracer.Emit(ctx, trace.Event{
			Tool:      name,
			Phase:     trace.PhaseRequest,
			Arguments: request.GetArguments(),
		})
		result, err := handler(ctx, request)
		event := trace.Event{
			Tool:       name,
			Phase:      trace.PhaseResponse,
			DurationMS: time.Since(start).Milliseconds(),
		}
		if err != nil {
			event.Phase = trace.PhaseError
			event.Error = err.Error()
			s.tracer.Emit(ctx, event)
			return result, err
		}
		if result != nil && result.IsError {
			event.Phase = trace.PhaseError
			event.Error = toolResultErrorText(result)
		} else if result != nil {
			event.Response = result.StructuredContent
		}
		s.tracer.Emit(ctx, event)
		return result, nil
	}
}

func toolResultErrorText(result *mcp.CallToolResult) string {
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			return text.Text
		}
	}
	return "tool returned an error"
}
