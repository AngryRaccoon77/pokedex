package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx-адаптер для database/sql, нужен goose
	"github.com/pressly/goose/v3"

	"pokedex-api/internal/config"
	"pokedex-api/internal/handlers"
	"pokedex-api/internal/middleware"
	"pokedex-api/internal/repository"
	"pokedex-api/internal/service"
	"pokedex-api/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting application initialization")

	// Паникует, если не заданы обязательные переменные окружения
	cfg := config.MustLoad()

	logger.Info("running database migrations...")
	if err := runMigrations(cfg.Postgres); err != nil {
		logger.Error("migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("database migrations completed successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := initPgxPool(ctx, cfg.Postgres)
	if err != nil {
		logger.Error("failed to initialize pgx pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("database connection pool initialized")

	// Сборка зависимостей снизу вверх
	pokemonRepo := repository.NewPostgresPokemonRepo(pool)
	pokemonService := service.NewPokemonService(pokemonRepo, logger)
	pokemonHandler := handlers.NewPokemonHandler(pokemonService, logger)

	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.StructuredLogger(logger))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))

	// AllowCredentials остаётся false: API не использует cookie-сессии,
	// аутентификация (если появится) будет на токенах в заголовке Authorization.
	// Поэтому широкий AllowedOrigins безопасен — credentialed-запросы не разрешены.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			logger.Error("failed to write health response", slog.String("error", err.Error()))
		}
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/pokemon", pokemonHandler.Routes())
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Ждём сигнал ОС или фатальную ошибку сервера
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	startupFailed := false
	select {
	case sig := <-quit:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	case err := <-serverErr:
		logger.Error("HTTP server failed to start", slog.String("error", err.Error()))
		startupFailed = true
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced server shutdown", slog.String("error", err.Error()))
	}

	if startupFailed {
		logger.Error("application stopped due to server startup failure")
		os.Exit(1)
	}

	logger.Info("application stopped gracefully")
}

// runMigrations применяет goose-миграции из встроенной файловой системы.
// Использует pgx stdlib-адаптер, так как goose работает с database/sql.
func runMigrations(cfg config.PostgresConfig) error {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return fmt.Errorf("open migration db: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.EmbedFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}

// initPgxPool создаёт и настраивает пул соединений pgxpool.
func initPgxPool(ctx context.Context, cfg config.PostgresConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}
