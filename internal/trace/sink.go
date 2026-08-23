package trace

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Sink receives trace events without affecting the caller's behavior.
type Sink interface {
	Emit(ctx context.Context, event Event)
}

// NoopSink discards all trace events.
type NoopSink struct{}

// Emit discards the event.
func (NoopSink) Emit(context.Context, Event) {}

// MetadataSink forwards only the fixed non-sensitive trace metadata allowlist.
type MetadataSink struct {
	inner Sink
}

// NewMetadataSink creates a sink that removes raw tool payloads and error text.
func NewMetadataSink(inner Sink) *MetadataSink {
	if inner == nil {
		inner = NoopSink{}
	}
	return &MetadataSink{inner: inner}
}

// Emit forwards only tool, phase, duration, session, process, status, and drop-count metadata.
func (s *MetadataSink) Emit(ctx context.Context, event Event) {
	s.inner.Emit(ctx, Event{
		SessionID:     event.SessionID,
		PID:           event.PID,
		Tool:          event.Tool,
		Phase:         event.Phase,
		DurationMS:    event.DurationMS,
		Status:        event.Status,
		DroppedEvents: event.DroppedEvents,
	})
}

// MultiSink fans events out to multiple sinks.
type MultiSink []Sink

// Emit forwards the event to each configured sink.
func (s MultiSink) Emit(ctx context.Context, event Event) {
	for _, sink := range s {
		if sink != nil {
			sink.Emit(ctx, event)
		}
	}
}

// AsyncSink decouples event production from sink delivery.
type AsyncSink struct {
	inner     Sink
	ch        chan Event
	once      sync.Once
	done      chan struct{}
	mu        sync.Mutex
	closed    bool
	dropped   atomic.Uint64
	sessionID string
	pid       int
}

// NewAsyncSink creates a non-blocking asynchronous sink.
func NewAsyncSink(inner Sink, bufferSize int) *AsyncSink {
	if inner == nil {
		inner = NoopSink{}
	}
	if bufferSize < 1 {
		bufferSize = 1
	}
	s := &AsyncSink{
		inner: inner,
		ch:    make(chan Event, bufferSize),
		done:  make(chan struct{}),
	}
	go s.run()
	return s
}

// Emit queues an event or drops it if the queue is full.
func (s *AsyncSink) Emit(ctx context.Context, event Event) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if event.SessionID != "" {
		s.sessionID = event.SessionID
	}
	if event.PID != 0 {
		s.pid = event.PID
	}
	select {
	case s.ch <- event:
	default:
		s.dropped.Add(1)
	}
}

// Dropped returns the number of events discarded because the queue was full.
func (s *AsyncSink) Dropped() uint64 {
	return s.dropped.Load()
}

// Close stops the worker after queued events are drained.
func (s *AsyncSink) Close() {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		if dropped := s.dropped.Load(); dropped > 0 {
			s.ch <- Event{
				SessionID:     s.sessionID,
				PID:           s.pid,
				Timestamp:     time.Now().UTC(),
				Tool:          "trace.queue",
				Phase:         PhaseHealth,
				Status:        StatusDegraded,
				DroppedEvents: dropped,
			}
		}
		close(s.ch)
		s.mu.Unlock()
		<-s.done
	})
}

func (s *AsyncSink) run() {
	defer close(s.done)
	for event := range s.ch {
		s.inner.Emit(context.Background(), event)
	}
}
