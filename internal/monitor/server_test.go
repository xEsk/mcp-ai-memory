package monitor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mcp-ai-memory/internal/trace"
)

func TestServerAcceptsEvents(t *testing.T) {
	server := NewServer(10)
	body := bytes.NewBufferString(`{"session_id":"s","tool":"memory.search","phase":"request"}`)
	request := httptest.NewRequest(http.MethodPost, "/events", body)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	events := server.Events()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Tool != "memory.search" {
		t.Fatalf("tool = %q, want memory.search", events[0].Tool)
	}
}

func TestServerKeepsOnlyRecentEvents(t *testing.T) {
	server := NewServer(2)

	for _, tool := range []string{"one", "two", "three"} {
		event := trace.Event{Tool: tool, Phase: trace.PhaseRequest}
		body, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body)))
	}

	events := server.Events()
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Tool != "two" || events[1].Tool != "three" {
		t.Fatalf("events = %v, want two and three", []string{events[0].Tool, events[1].Tool})
	}
}

func TestServerStreamsEvents(t *testing.T) {
	server := NewServer(10)
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	responses := make(chan string, 1)
	go func() {
		response, err := http.Get(httpServer.URL + "/events/stream")
		if err != nil {
			responses <- err.Error()
			return
		}
		defer response.Body.Close()
		reader := bufio.NewReader(response.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				responses <- err.Error()
				return
			}
			if strings.HasPrefix(line, "data: ") {
				responses <- line
				return
			}
		}
	}()

	time.Sleep(10 * time.Millisecond)
	body := bytes.NewBufferString(`{"session_id":"s","tool":"memory.search","phase":"request"}`)
	response, err := http.Post(httpServer.URL+"/events", "application/json", body)
	if err != nil {
		t.Fatalf("post event: %v", err)
	}
	response.Body.Close()

	select {
	case line := <-responses:
		if !strings.Contains(line, `"tool":"memory.search"`) {
			t.Fatalf("stream line = %q, want memory.search event", line)
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not receive event")
	}
}
