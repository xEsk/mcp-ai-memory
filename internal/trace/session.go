package trace

import (
	"context"
	"time"
)

// SessionSink adds process session metadata to emitted trace events.
type SessionSink struct {
	sessionID string
	pid       int
	inner     Sink
}

// NewSessionSink creates a sink that stamps events with session metadata.
func NewSessionSink(sessionID string, pid int, inner Sink) *SessionSink {
	if inner == nil {
		inner = NoopSink{}
	}
	return &SessionSink{sessionID: sessionID, pid: pid, inner: inner}
}

// Emit fills missing session fields and forwards the event.
func (s *SessionSink) Emit(ctx context.Context, event Event) {
	if event.SessionID == "" {
		event.SessionID = s.sessionID
	}
	if event.PID == 0 {
		event.PID = s.pid
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	s.inner.Emit(ctx, event)
}
