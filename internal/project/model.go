package project

import "time"

// Project represents an isolated memory space.
type Project struct {
	ID          string     `json:"id"`
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// Stats summarizes one project's stored memories for management views.
type Stats struct {
	ActiveMemories  int
	DeletedMemories int
	TotalMemories   int
	Retrievals      int
	NeverRetrieved  int
	LatestMemoryAt  *time.Time
	MemoriesByType  map[string]int
}

// CleanupStats describes data eligible for permanent cleanup.
type CleanupStats struct {
	DeletedProjects        int
	DeletedProjectMemories int
	DeletedMemories        int
	StaleFTS               int
	OrphanedEnrichments    int
}

// UpsertInput contains the data needed to create or update a project.
type UpsertInput struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
