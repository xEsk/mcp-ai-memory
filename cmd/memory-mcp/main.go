package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"mcp-ai-memory/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	if code != 0 {
		os.Exit(code)
	}
}

func run(ctx context.Context, args []string, input io.Reader, output, errorsOutput io.Writer) int {
	mode, dbPath, err := parseCommand(args, os.Getenv("MEMORY_DB_PATH"), errorsOutput)
	if err != nil {
		fmt.Fprintln(errorsOutput, err)
		return 2
	}
	if mode == "manage" {
		application, err := app.NewManagement(ctx, dbPath, input, output, terminalStyled(output))
		if err != nil {
			fmt.Fprintf(errorsOutput, "starting memory manager: %v\n", err)
			return 1
		}
		defer func() {
			if err := application.Close(); err != nil {
				fmt.Fprintf(errorsOutput, "closing memory manager: %v\n", err)
			}
		}()
		if err := application.Runner.Run(ctx); err != nil {
			fmt.Fprintf(errorsOutput, "running memory manager: %v\n", err)
			return 1
		}
		return 0
	}

	application, err := app.New(ctx, app.LoadConfig())
	if err != nil {
		slog.New(slog.NewTextHandler(errorsOutput, nil)).Error("starting memory mcp", "error", err)
		return 1
	}
	defer func() {
		if err := application.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing application: %v\n", err)
		}
	}()

	if err := application.Server.ServeStdio(ctx); err != nil {
		slog.New(slog.NewTextHandler(errorsOutput, nil)).Error("serving memory mcp", "error", err)
		return 1
	}
	return 0
}

func parseCommand(args []string, envDBPath string, errorsOutput io.Writer) (mode, dbPath string, err error) {
	if len(args) == 0 {
		return "serve", envDBPath, nil
	}
	if args[0] != "manage" {
		return "", "", fmt.Errorf("unknown command %q; use memory-mcp or memory-mcp manage", args[0])
	}
	flags := flag.NewFlagSet("memory-mcp manage", flag.ContinueOnError)
	flags.SetOutput(errorsOutput)
	path := flags.String("db", envDBPath, "absolute path to the memory SQLite database")
	if err := flags.Parse(args[1:]); err != nil {
		return "", "", err
	}
	if flags.NArg() != 0 {
		return "", "", fmt.Errorf("unexpected manage arguments: %s", strings.Join(flags.Args(), " "))
	}
	return "manage", *path, nil
}

func terminalStyled(output io.Writer) bool {
	file, ok := output.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
