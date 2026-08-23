package trace

import "time"

const (
	PhaseRequest  = "request"
	PhaseResponse = "response"
	PhaseError    = "error"
	PhaseHealth   = "health"

	StatusStarted        = "started"
	StatusSuccess        = "success"
	StatusToolError      = "tool_error"
	StatusTechnicalError = "technical_error"
	StatusDegraded       = "degraded"
)

// Event describes one observed MCP tool call phase.
type Event struct {
	SessionID     string    `json:"session_id"`
	PID           int       `json:"pid"`
	Timestamp     time.Time `json:"timestamp,omitzero"`
	Tool          string    `json:"tool"`
	Phase         string    `json:"phase"`
	DurationMS    int64     `json:"duration_ms,omitempty"`
	Status        string    `json:"status,omitempty"`
	DroppedEvents uint64    `json:"dropped_events,omitempty"`
	Arguments     any       `json:"arguments,omitempty"`
	Response      any       `json:"response,omitempty"`
	Error         string    `json:"error,omitempty"`
}
