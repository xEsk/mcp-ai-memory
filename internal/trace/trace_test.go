package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
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

	sink, err := NewMonitorSink(server.URL+"/events", 100*time.Millisecond, false)
	if err != nil {
		t.Fatalf("new monitor sink: %v", err)
	}
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
	sink, err := NewMonitorSink("http://127.0.0.1:1/events", time.Millisecond, false)
	if err != nil {
		t.Fatalf("new monitor sink: %v", err)
	}

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

	sink, err := NewMonitorSink(server.URL+"/events", time.Millisecond, false)
	if err != nil {
		t.Fatalf("new monitor sink: %v", err)
	}
	start := time.Now()
	sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("monitor sink took %s, want short timeout", elapsed)
	}
}

func TestMonitorSinkRejectsNonLoopbackEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"http://example.com/events",
		"http://192.0.2.1/events",
		"http://[2001:db8::1]/events",
		"http://localhost.example/events",
	} {
		t.Run(endpoint, func(t *testing.T) {
			if _, err := NewMonitorSink(endpoint, time.Second, false); err == nil {
				t.Fatalf("NewMonitorSink(%q) succeeded", endpoint)
			}
		})
	}
}

func TestMonitorSinkAcceptsLiteralLoopbackAndExactLocalhostEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"http://localhost:7379/events",
		"http://127.0.0.1:7379/events",
		"http://[::1]:7379/events",
	} {
		t.Run(endpoint, func(t *testing.T) {
			if _, err := NewMonitorSink(endpoint, time.Second, false); err != nil {
				t.Fatalf("NewMonitorSink(%q): %v", endpoint, err)
			}
		})
	}
}

func TestMonitorSinkDoesNotFollowRedirects(t *testing.T) {
	targetHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetHits++
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/events", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	sink, err := NewMonitorSink(redirect.URL+"/events", time.Second, false)
	if err != nil {
		t.Fatalf("new monitor sink: %v", err)
	}
	sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})

	if targetHits != 0 {
		t.Fatalf("redirect target hits = %d, want 0", targetHits)
	}
}

func TestMonitorSinkRejectsNonLoopbackEffectiveDialAddress(t *testing.T) {
	connection := &stubConn{remote: &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 7379}}
	sink, err := newMonitorSink(
		"http://localhost:7379/events",
		time.Second,
		false,
		func(context.Context, string, string) (net.Conn, error) { return connection, nil },
	)
	if err != nil {
		t.Fatalf("new monitor sink: %v", err)
	}

	sink.Emit(context.Background(), Event{Tool: "memory.search", Phase: PhaseRequest})

	if !connection.closed {
		t.Fatal("non-loopback connection was not closed")
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

type stubConn struct {
	closed bool
	remote net.Addr
}

func (*stubConn) Read([]byte) (int, error)         { return 0, errors.New("not implemented") }
func (*stubConn) Write([]byte) (int, error)        { return 0, errors.New("not implemented") }
func (c *stubConn) Close() error                   { c.closed = true; return nil }
func (*stubConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c *stubConn) RemoteAddr() net.Addr           { return c.remote }
func (*stubConn) SetDeadline(time.Time) error      { return nil }
func (*stubConn) SetReadDeadline(time.Time) error  { return nil }
func (*stubConn) SetWriteDeadline(time.Time) error { return nil }
