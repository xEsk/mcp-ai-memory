package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	"mcp-ai-memory/internal/enrich"
	"mcp-ai-memory/internal/manage"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/storage"
)

// ManagementApp owns an exclusive offline management session.
type ManagementApp struct {
	DB              *sql.DB
	Runner          *manage.Runner
	maintenanceLock *storage.OperationLock
}

// NewManagement opens the database exclusively and wires the terminal manager.
func NewManagement(ctx context.Context, dbPath string, input io.Reader, output io.Writer, styled bool) (*ManagementApp, error) {
	if err := validateDBPath(dbPath); err != nil {
		return nil, err
	}
	lock, err := storage.AcquireMaintenanceLock(dbPath)
	if err != nil {
		return nil, err
	}
	db, err := storage.OpenSQLite(ctx, dbPath)
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	if err := lock.BindDatabaseIdentity(dbPath); err != nil {
		_ = db.Close()
		_ = lock.Close()
		return nil, err
	}
	clock := platform.NewClock()
	projects := project.NewService(project.NewRepository(db), clock)
	memories := memory.NewService(memory.NewRepository(db), projects, enrich.NewLocalHeuristicEnricher(), clock)
	return &ManagementApp{
		DB:              db,
		Runner:          manage.New(projects, memories, input, output, styled),
		maintenanceLock: lock,
	}, nil
}

// Close releases the management database and exclusive operation lock.
func (a *ManagementApp) Close() error {
	var dbErr error
	if a.DB != nil {
		dbErr = a.DB.Close()
	}
	var lockErr error
	if a.maintenanceLock != nil {
		lockErr = a.maintenanceLock.Close()
	}
	if dbErr != nil {
		return fmt.Errorf("closing management database: %w", dbErr)
	}
	if lockErr != nil {
		return fmt.Errorf("releasing management lock: %w", lockErr)
	}
	return nil
}
