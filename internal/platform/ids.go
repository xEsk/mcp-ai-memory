package platform

import "github.com/google/uuid"

// NewID generates opaque identifiers for persisted entities.
func NewID() string {
	return uuid.NewString()
}
