package mcpserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleServerInfoReturnsBasicVersion(t *testing.T) {
	server := &Server{}

	result, err := server.handleServerInfo(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("server info: %v", err)
	}
	if result.IsError {
		t.Fatal("server info returned tool error")
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}
	if content["name"] != serverName {
		t.Fatalf("name = %q, want %q", content["name"], serverName)
	}
	if content["version"] != serverVersion {
		t.Fatalf("version = %q, want %q", content["version"], serverVersion)
	}
	if len(content) != 2 {
		t.Fatalf("field count = %d, want 2", len(content))
	}
}
