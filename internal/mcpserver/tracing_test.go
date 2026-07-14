package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"mcp-ai-memory/internal/trace"
)

func TestTraceToolMetadataContainsOnlyAllowlistedFields(t *testing.T) {
	recorder := &recordingTraceSink{}
	server := &Server{tracer: trace.NewSessionSink("session-1", 42, trace.NewMetadataSink(recorder))}
	handler := server.traceTool("memory.search", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultStructuredOnly(map[string]any{"content": "sentinel-response"}), nil
	})

	_, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"project": "sentinel-project",
			"query":   "sentinel-query",
			"content": "sentinel-content",
		}},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if len(recorder.events) != 2 {
		t.Fatalf("events = %d, want 2", len(recorder.events))
	}

	for _, event := range recorder.events {
		payload, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			t.Fatalf("marshal event: %v", marshalErr)
		}
		encoded := string(payload)
		for _, sentinel := range []string{"sentinel-project", "sentinel-query", "sentinel-content", "sentinel-response"} {
			if strings.Contains(encoded, sentinel) {
				t.Fatalf("metadata payload contains %q: %s", sentinel, encoded)
			}
		}
		var fields map[string]any
		if unmarshalErr := json.Unmarshal(payload, &fields); unmarshalErr != nil {
			t.Fatalf("unmarshal event: %v", unmarshalErr)
		}
		for key := range fields {
			switch key {
			case "session_id", "pid", "tool", "phase", "duration_ms", "status":
			default:
				t.Fatalf("metadata payload contains non-allowlisted field %q: %s", key, encoded)
			}
		}
	}
}

func TestTraceToolMetadataClassifiesErrorsWithoutRawText(t *testing.T) {
	recorder := &recordingTraceSink{}
	server := &Server{tracer: trace.NewMetadataSink(recorder)}
	handler := server.traceTool("memory.get", func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, errors.New("sentinel raw database error")
	})

	_, _ = handler(context.Background(), mcp.CallToolRequest{})

	if got := recorder.events[1].Status; got != trace.StatusTechnicalError {
		t.Fatalf("status = %q, want %q", got, trace.StatusTechnicalError)
	}
	if recorder.events[1].Error != "" {
		t.Fatalf("metadata error = %q, want empty", recorder.events[1].Error)
	}
}

func TestTraceToolEmitsRequestAndResponse(t *testing.T) {
	sink := &recordingTraceSink{}
	server := &Server{tracer: trace.NewSessionSink("session-1", 42, sink)}
	handler := server.traceTool("memory.search", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultStructuredOnly(map[string]any{"ok": true}), nil
	})

	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"project": "api"}},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatal("handler returned tool error")
	}
	if len(sink.events) != 2 {
		t.Fatalf("events = %d, want 2", len(sink.events))
	}
	if sink.events[0].Phase != trace.PhaseRequest {
		t.Fatalf("first phase = %q, want request", sink.events[0].Phase)
	}
	if sink.events[0].Status != trace.StatusStarted {
		t.Fatalf("first status = %q, want started", sink.events[0].Status)
	}
	arguments, ok := sink.events[0].Arguments.(map[string]any)
	if !ok || arguments["project"] != "api" {
		t.Fatalf("raw request arguments = %#v, want project payload", sink.events[0].Arguments)
	}
	if sink.events[1].Phase != trace.PhaseResponse {
		t.Fatalf("second phase = %q, want response", sink.events[1].Phase)
	}
	if sink.events[1].Status != trace.StatusSuccess {
		t.Fatalf("second status = %q, want success", sink.events[1].Status)
	}
	if sink.events[1].Response == nil {
		t.Fatal("response event did not include response payload")
	}
}

func TestTraceToolRecordsToolErrorsWithoutChangingResult(t *testing.T) {
	sink := &recordingTraceSink{}
	server := &Server{tracer: trace.NewSessionSink("session-1", 42, sink)}
	handler := server.traceTool("memory.get", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("not found"), nil
	})

	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Fatal("handler result should remain a tool error")
	}
	if len(sink.events) != 2 {
		t.Fatalf("events = %d, want 2", len(sink.events))
	}
	if sink.events[1].Phase != trace.PhaseError {
		t.Fatalf("second phase = %q, want error", sink.events[1].Phase)
	}
	if sink.events[1].Status != trace.StatusToolError {
		t.Fatalf("second status = %q, want tool_error", sink.events[1].Status)
	}
	if sink.events[1].Error == "" {
		t.Fatal("tool error event did not include error text")
	}
}

func TestTraceToolRecordsTechnicalErrorsWithoutChangingError(t *testing.T) {
	sink := &recordingTraceSink{}
	server := &Server{tracer: trace.NewSessionSink("session-1", 42, sink)}
	expected := errors.New("database unavailable")
	handler := server.traceTool("project.list", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, expected
	})

	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v, want %v", err, expected)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
	if len(sink.events) != 2 {
		t.Fatalf("events = %d, want 2", len(sink.events))
	}
	if sink.events[1].Phase != trace.PhaseError {
		t.Fatalf("second phase = %q, want error", sink.events[1].Phase)
	}
	if sink.events[1].Status != trace.StatusTechnicalError {
		t.Fatalf("second status = %q, want technical_error", sink.events[1].Status)
	}
	if sink.events[1].Error != "database unavailable" {
		t.Fatalf("error text = %q, want database unavailable", sink.events[1].Error)
	}
}

type recordingTraceSink struct {
	events []trace.Event
}

func (s *recordingTraceSink) Emit(_ context.Context, event trace.Event) {
	s.events = append(s.events, event)
}
