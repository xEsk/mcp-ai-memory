package memory

import (
	"errors"
	"strings"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("memory not found")
)

var validTypes = map[string]bool{
	TypeArchitecture:   true,
	TypeDecision:       true,
	TypeAPI:            true,
	TypeImplementation: true,
	TypeBug:            true,
	TypeWorkflow:       true,
	TypeConvention:     true,
	TypeDomain:         true,
	TypeOther:          true,
}

// NormalizeType validates the memory type and uses "other" when none is specified.
func NormalizeType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return TypeOther, nil
	}
	if !validTypes[value] {
		return "", ErrInvalidInput
	}
	return value, nil
}

// IsValidType indicates whether a type belongs to the supported vocabulary.
func IsValidType(value string) bool {
	return validTypes[value]
}
