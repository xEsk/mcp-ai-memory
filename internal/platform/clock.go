package platform

import "time"

// Clock allows tests to replace the time source.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

// NewClock creates a clock backed by the system clock.
func NewClock() Clock {
	return systemClock{}
}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}
