// Package core holds the small set of types and helpers shared across every
// domain package (game, quiz, question): sentinel errors, pgtype
// conversions, and the storage dependency's narrow interface. It knows
// nothing about HTTP, websockets, or any single domain's business rules.
package core

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrValidation = errors.New("validation")
)
