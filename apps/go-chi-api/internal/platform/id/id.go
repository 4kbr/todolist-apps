// Package id membungkus strategi pembuatan ID (UUIDv7) supaya kalau
// strateginya berubah, titik ubahnya cuma satu tempat.
package id

import (
	"fmt"

	"github.com/google/uuid"
)

// New generate UUID v7
func New() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate id: %w", err)
	}
	return id, nil
}

// MustNew generate UUID v7, panic jika error
func MustNew() uuid.UUID {
	id, err := New()
	if err != nil {
		panic(err)
	}
	return id
}
