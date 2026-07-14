package trace

import (
	"context"
	"sync"
)

// Sink receives trace events without affecting the caller's behavior.
type Sink interface {
	Emit(ctx context.Context, event Event)
}

// NoopSink discards all trace events.
type NoopSink struct{}

// Emit discards the event.
func (NoopSink) Emit(context.Context, Event) {}

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
	inner Sink
	ch    chan Event
	once  sync.Once
	done  chan struct{}
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
	select {
	case s.ch <- event:
	default:
	}
}

// Close stops the worker after queued events are drained.
func (s *AsyncSink) Close() {
	s.once.Do(func() {
		close(s.ch)
		<-s.done
	})
}

func (s *AsyncSink) run() {
	defer close(s.done)
	for event := range s.ch {
		s.inner.Emit(context.Background(), event)
	}
}
