package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// MonitorSink sends trace events to a local monitor endpoint.
type MonitorSink struct {
	client  *http.Client
	url     string
	timeout time.Duration
	debug   bool
}

// NewMonitorSink creates a fail-silent HTTP sink for local monitor events.
func NewMonitorSink(url string, timeout time.Duration, debug bool) *MonitorSink {
	if timeout <= 0 {
		timeout = 25 * time.Millisecond
	}
	return &MonitorSink{
		client:  &http.Client{Timeout: timeout},
		url:     url,
		timeout: timeout,
		debug:   debug,
	}
}

// Emit posts one event to the monitor and absorbs all errors.
func (s *MonitorSink) Emit(ctx context.Context, event Event) {
	if s == nil || s.url == "" {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		s.debugError("marshal trace event: %v", err)
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, s.url, bytes.NewReader(payload))
	if err != nil {
		s.debugError("create trace request: %v", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		s.debugError("send trace event: %v", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.debugError("send trace event: status %s", response.Status)
	}
}

func (s *MonitorSink) debugError(format string, args ...any) {
	if !s.debug {
		return
	}
	fmt.Fprintf(os.Stderr, "trace: "+format+"\n", args...)
}
