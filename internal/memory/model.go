package memory

import "time"

const (
	TypeArchitecture   = "architecture"
	TypeDecision       = "decision"
	TypeAPI            = "api"
	TypeImplementation = "implementation"
	TypeBug            = "bug"
	TypeWorkflow       = "workflow"
	TypeConvention     = "convention"
	TypeDomain         = "domain"
	TypeOther          = "other"
)

// Memory contains the original text and enriched metadata for search.
type Memory struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	ProjectKey     string     `json:"project"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	Summary        string     `json:"summary"`
	ShortSummary   string     `json:"short_summary"`
	Keywords       []string   `json:"keywords"`
	Tags           []string   `json:"tags"`
	Entities       []string   `json:"entities"`
	Aliases        []string   `json:"aliases"`
	SearchPhrases  []string   `json:"search_phrases"`
	Importance     int        `json:"importance"`
	Source         string     `json:"source"`
	AccessCount    int        `json:"access_count"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// UpsertInput contains the memory create or update request.
type UpsertInput struct {
	Project    string   `json:"project"`
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	Importance int      `json:"importance"`
	Source     string   `json:"source"`
	Enrich     bool     `json:"enrich"`
}

// ListInput filters the memory list by project and simple metadata.
type ListInput struct {
	Project string `json:"project"`
	Type    string `json:"type"`
	Tag     string `json:"tag"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}
