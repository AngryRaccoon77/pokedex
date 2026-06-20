// Package repository определяет интерфейс доступа к данным.
// Service-слой зависит от интерфейса, а не от конкретной реализации (PostgreSQL).
package repository

import (
	"context"

	"pokedex-api/internal/domain"
)

type PokemonRepository interface {
	GetByID(ctx context.Context, id int64) (domain.Pokemon, error)
	List(ctx context.Context, filter domain.PokemonFilter) ([]domain.Pokemon, error)
	Create(ctx context.Context, input domain.CreatePokemonInput) (domain.Pokemon, error)
	Update(ctx context.Context, id int64, input domain.UpdatePokemonInput) (domain.Pokemon, error)
	Delete(ctx context.Context, id int64) error
}
