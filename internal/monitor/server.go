package monitor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"mcp-ai-memory/internal/trace"
)

// Server receives trace events and serves a small real-time monitor UI.
type Server struct {
	mu          sync.Mutex
	events      []trace.Event
	limit       int
	subscribers map[chan trace.Event]struct{}
}

// NewServer creates a monitor server that retains the most recent events.
func NewServer(limit int) *Server {
	if limit < 1 {
		limit = 1000
	}
	return &Server{
		limit:       limit,
		subscribers: map[chan trace.Event]struct{}{},
	}
}

// ServeHTTP routes monitor UI, event ingestion, and SSE streaming.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		s.handleIndex(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/events":
		s.handleEvents(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/events/stream":
		s.handleStream(w, r)
	default:
		http.NotFound(w, r)
	}
}

// Events returns a snapshot of retained trace events.
func (s *Server) Events() []trace.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]trace.Event, len(s.events))
	copy(events, s.events)
	return events
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var event trace.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid event", http.StatusBadRequest)
		return
	}
	s.appendEvent(event)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) appendEvent(event trace.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	if len(s.events) > s.limit {
		s.events = append([]trace.Event(nil), s.events[len(s.events)-s.limit:]...)
	}
	for subscriber := range s.subscribers {
		select {
		case subscriber <- event:
		default:
			if event.Phase == trace.PhaseHealth {
				select {
				case <-subscriber:
				default:
				}
				select {
				case subscriber <- event:
				default:
				}
			}
		}
	}
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events := s.Events()
	for _, event := range events {
		writeSSE(w, event)
	}
	flusher.Flush()

	ch := make(chan trace.Event, 32)
	s.addSubscriber(ch)
	defer s.removeSubscriber(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func (s *Server) addSubscriber(ch chan trace.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[ch] = struct{}{}
}

func (s *Server) removeSubscriber(ch chan trace.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, ch)
	close(ch)
}

func writeSSE(w http.ResponseWriter, event trace.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", payload)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}
