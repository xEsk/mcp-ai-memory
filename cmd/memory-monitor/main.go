package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"mcp-ai-memory/internal/monitor"
)

func main() {
	addr := "127.0.0.1:7379"
	server := monitor.NewServer(1000)
	fmt.Printf("memory monitor listening on http://%s\n", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		slog.Error("serving memory monitor", "error", err)
	}
}
