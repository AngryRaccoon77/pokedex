package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pokedex-api/internal/domain"
)

// postgresPokemonRepo реализует PokemonRepository для PostgreSQL через pgxpool.
// Пул потокобезопасен, один экземпляр на всё приложение.
type postgresPokemonRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresPokemonRepo(pool *pgxpool.Pool) PokemonRepository {
	return &postgresPokemonRepo{pool: pool}
}

func (r *postgresPokemonRepo) GetByID(ctx context.Context, id int64) (domain.Pokemon, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, power, type, catchable, created_at, updated_at
		FROM pokemon
		WHERE id = $1`, id,
	)
	if err != nil {
		return domain.Pokemon{}, fmt.Errorf("query pokemon by id: %w", err)
	}
	defer rows.Close()

	pokemon, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[domain.Pokemon])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Pokemon{}, domain.ErrNotFound
		}
		return domain.Pokemon{}, fmt.Errorf("scan pokemon: %w", err)
	}

	return pokemon, nil
}

func (r *postgresPokemonRepo) List(ctx context.Context, filter domain.PokemonFilter) ([]domain.Pokemon, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Динамическая сборка WHERE из переданных фильтров
	query := `
		SELECT id, name, description, power, type, catchable, created_at, updated_at
		FROM pokemon
		WHERE 1=1`
	args := []any{}
	argIdx := 1

	if filter.CatchableOnly {
		query += fmt.Sprintf(" AND catchable = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}

	if filter.Search != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pokemon: %w", err)
	}
	defer rows.Close()

	pokemons, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Pokemon])
	if err != nil {
		return nil, fmt.Errorf("scan pokemon list: %w", err)
	}

	// Гарантируем [] вместо null в JSON
	if pokemons == nil {
		pokemons = []domain.Pokemon{}
	}

	return pokemons, nil
}

func (r *postgresPokemonRepo) Create(ctx context.Context, input domain.CreatePokemonInput) (domain.Pokemon, error) {
	var pokemon domain.Pokemon

	// INSERT ... RETURNING даёт всю строку за один round-trip.
	// type объявлен NOT NULL без DEFAULT — обязательно включаем его в колонки и параметры.
	err := r.pool.QueryRow(ctx, `
		INSERT INTO pokemon (name, description, power, type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, power, type, catchable, created_at, updated_at`,
		input.Name, input.Description, input.Power, input.Type,
	).Scan(
		&pokemon.ID,
		&pokemon.Name,
		&pokemon.Description,
		&pokemon.Power,
		&pokemon.Type,
		&pokemon.Catchable,
		&pokemon.CreatedAt,
		&pokemon.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Pokemon{}, domain.ErrDuplicateName
		}
		return domain.Pokemon{}, fmt.Errorf("insert pokemon: %w", err)
	}

	return pokemon, nil
}

func (r *postgresPokemonRepo) Update(ctx context.Context, id int64, input domain.UpdatePokemonInput) (domain.Pokemon, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Pokemon{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Проверяем, что запись существует
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pokemon WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return domain.Pokemon{}, fmt.Errorf("check pokemon exists: %w", err)
	}
	if !exists {
		return domain.Pokemon{}, domain.ErrNotFound
	}

	// COALESCE оставляет текущее значение, если передан nil
	var pokemon domain.Pokemon
	err = tx.QueryRow(ctx, `
		UPDATE pokemon
		SET name        = COALESCE($2, name),
		    description = COALESCE($3, description),
		    power       = COALESCE($4, power),
		    type        = COALESCE($5, type),
		    catchable   = COALESCE($6, catchable),
		    updated_at  = NOW()
		WHERE id = $1
		RETURNING id, name, description, power, type, catchable, created_at, updated_at`,
		id, input.Name, input.Description, input.Power, input.Type, input.Catchable,
	).Scan(
		&pokemon.ID,
		&pokemon.Name,
		&pokemon.Description,
		&pokemon.Power,
		&pokemon.Type,
		&pokemon.Catchable,
		&pokemon.CreatedAt,
		&pokemon.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Pokemon{}, domain.ErrDuplicateName
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Pokemon{}, domain.ErrNotFound
		}
		return domain.Pokemon{}, fmt.Errorf("update pokemon: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Pokemon{}, fmt.Errorf("commit tx: %w", err)
	}

	return pokemon, nil
}

func (r *postgresPokemonRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM pokemon WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete pokemon: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
