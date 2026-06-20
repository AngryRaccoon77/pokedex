// Package service содержит бизнес-логику - валидация, оркестрация репозитория, логирование.
package service

import (
	"context"
	"fmt"
	"log/slog"

	"pokedex-api/internal/domain"
	"pokedex-api/internal/repository"
)

type PokemonService struct {
	repo   repository.PokemonRepository
	logger *slog.Logger
}

func NewPokemonService(repo repository.PokemonRepository, logger *slog.Logger) *PokemonService {
	return &PokemonService{repo: repo, logger: logger}
}

func (s *PokemonService) GetByID(ctx context.Context, id int64) (domain.Pokemon, error) {
	pokemon, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Pokemon{}, fmt.Errorf("service get pokemon: %w", err)
	}
	return pokemon, nil
}

func (s *PokemonService) List(ctx context.Context, filter domain.PokemonFilter) ([]domain.Pokemon, error) {
	pokemons, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("service list pokemons: %w", err)
	}
	return pokemons, nil
}

func (s *PokemonService) Create(ctx context.Context, input domain.CreatePokemonInput) (domain.Pokemon, error) {
	if err := input.Validate(); err != nil {
		return domain.Pokemon{}, err
	}

	pokemon, err := s.repo.Create(ctx, input)
	if err != nil {
		return domain.Pokemon{}, fmt.Errorf("service create pokemon: %w", err)
	}

	s.logger.Info("pokemon created",
		slog.Int64("id", pokemon.ID),
		slog.String("name", pokemon.Name),
	)
	return pokemon, nil
}

func (s *PokemonService) Update(ctx context.Context, id int64, input domain.UpdatePokemonInput) (domain.Pokemon, error) {
	if err := input.Validate(); err != nil {
		return domain.Pokemon{}, err
	}

	pokemon, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return domain.Pokemon{}, fmt.Errorf("service update pokemon: %w", err)
	}

	s.logger.Info("pokemon updated", slog.Int64("id", pokemon.ID))
	return pokemon, nil
}

func (s *PokemonService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("service delete pokemon: %w", err)
	}
	s.logger.Info("pokemon deleted", slog.Int64("id", id))
	return nil
}
