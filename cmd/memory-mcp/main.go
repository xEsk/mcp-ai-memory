package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"mcp-ai-memory/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	application, err := app.New(ctx, app.LoadConfig())
	if err != nil {
		slog.Error("starting memory mcp", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := application.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing application: %v\n", err)
		}
	}()

	if err := application.Server.ServeStdio(ctx); err != nil {
		slog.Error("serving memory mcp", "error", err)
		os.Exit(1)
	}
}
