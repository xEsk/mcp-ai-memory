package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNoopSinkEmitDoesNotFail(t *testing.T) {
	sink := NoopSink{}

	sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})
}

func TestAsyncSinkDropsWhenBufferIsFull(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	sink := NewAsyncSink(&blockingSink{blocked: blocked, release: release}, 1)
	defer sink.Close()

	sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})
	<-blocked

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseResponse})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("async sink blocked while buffer was full")
	}

	close(release)
}

func TestMonitorSinkPostsEvents(t *testing.T) {
	received := make(chan Event, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/events" {
			t.Errorf("path = %s, want /events", r.URL.Path)
		}
		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("decode event: %v", err)
		}
		received <- event
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sink := NewMonitorSink(server.URL+"/events", 100*time.Millisecond, false)
	sink.Emit(context.Background(), Event{
		SessionID: "session-1",
		Tool:      "memory.search",
		Phase:     PhaseRequest,
		Arguments: map[string]any{"project": "api"},
	})

	select {
	case event := <-received:
		if event.Tool != "memory.search" {
			t.Fatalf("tool = %q, want memory.search", event.Tool)
		}
		if event.SessionID != "session-1" {
			t.Fatalf("session id = %q, want session-1", event.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("monitor did not receive event")
	}
}

func TestMonitorSinkIgnoresConnectionErrors(t *testing.T) {
	sink := NewMonitorSink("http://127.0.0.1:1/events", time.Millisecond, false)

	done := make(chan struct{})
	go func() {
		sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("monitor sink blocked on connection error")
	}
}

func TestMonitorSinkIgnoresTimeouts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sink := NewMonitorSink(server.URL+"/events", time.Millisecond, false)
	start := time.Now()
	sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("monitor sink took %s, want short timeout", elapsed)
	}
}

func TestMultiSinkEmitsToAllSinks(t *testing.T) {
	first := &recordingSink{}
	second := &recordingSink{}
	sink := MultiSink{first, second}

	sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})

	if len(first.events) != 1 {
		t.Fatalf("first sink events = %d, want 1", len(first.events))
	}
	if len(second.events) != 1 {
		t.Fatalf("second sink events = %d, want 1", len(second.events))
	}
}

type blockingSink struct {
	blocked chan struct{}
	once    sync.Once
	release chan struct{}
}

func (s *blockingSink) Emit(context.Context, Event) {
	s.once.Do(func() { close(s.blocked) })
	<-s.release
}

type recordingSink struct {
	events []Event
}

func (s *recordingSink) Emit(_ context.Context, event Event) {
	s.events = append(s.events, event)
}

func TestMonitorSinkJSONShape(t *testing.T) {
	event := Event{SessionID: "s", Tool: "memory.get", Phase: PhaseResponse}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(event); err != nil {
		t.Fatalf("encode event: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"session_id":"s"`)) {
		t.Fatalf("encoded event does not include session_id: %s", buf.String())
	}
}
