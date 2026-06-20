// Package domain содержит бизнес-модели и доменные ошибки.
// Никаких зависимостей от HTTP, БД или внешних библиотек.
package domain

import (
	"errors"
	"time"
)

// Pokemon — порядок полей соответствует порядку колонок в SELECT
// (id, name, description, power, type, catchable, created_at, updated_at),
// что требуется для pgx.RowToStructByPos.
type Pokemon struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Power       int       `json:"power"`
	Type        string    `json:"type"`
	Catchable   bool      `json:"catchable"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreatePokemonInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Power       int    `json:"power"`
	Type        string `json:"type"`
}

func (c CreatePokemonInput) Validate() error {
	if c.Name == "" {
		return ErrNameRequired
	}
	if c.Power < 0 {
		return ErrNegativePower
	}
	if !IsValidType(c.Type) {
		return ErrInvalidType
	}
	return nil
}

// UpdatePokemonInput использует указатели, чтобы отличить непереданное поле (nil) от явно заданного значения.
type UpdatePokemonInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Power       *int    `json:"power,omitempty"`
	Type        *string `json:"type,omitempty"`
	Catchable   *bool   `json:"catchable,omitempty"`
}

func (u UpdatePokemonInput) Validate() error {
	if u.Name != nil && *u.Name == "" {
		return ErrNameRequired
	}
	if u.Power != nil && *u.Power < 0 {
		return ErrNegativePower
	}
	if u.Type != nil && !IsValidType(*u.Type) {
		return ErrInvalidType
	}
	return nil
}

// PokemonFilter задаёт фильтрацию и пагинацию для списка покемонов.
type PokemonFilter struct {
	CatchableOnly bool
	Type          string
	Search        string
	Limit         int
	Offset        int
}

// validTypes — допустимые значения поля Type.
var validTypes = map[string]struct{}{
	"fire":     {},
	"water":    {},
	"grass":    {},
	"electric": {},
	"psychic":  {},
	"normal":   {},
	"ice":      {},
	"dragon":   {},
	"dark":     {},
	"fairy":    {},
	"fighting": {},
	"poison":   {},
	"ground":   {},
	"flying":   {},
	"bug":      {},
	"rock":     {},
	"ghost":    {},
	"steel":    {},
}

// IsValidType сообщает, входит ли t в множество допустимых типов покемона.
func IsValidType(t string) bool {
	_, ok := validTypes[t]
	return ok
}

// Доменные ошибки. Handler'ы мапят их на HTTP-коды.
var (
	ErrNotFound      = errors.New("pokemon not found")
	ErrDuplicateName = errors.New("pokemon with this name already exists")
	ErrNameRequired  = errors.New("name is required")
	ErrNegativePower = errors.New("power must be non-negative")
	ErrInvalidType   = errors.New("invalid pokemon type")
)
