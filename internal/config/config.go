// Package config загружает конфигурацию приложения из переменных окружения.
// Конфигурация читается один раз при старте и передаётся явно, без глобальных переменных.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
}

type ServerConfig struct {
	Port         int           // SERVER_PORT
	ReadTimeout  time.Duration // SERVER_READ_TIMEOUT
	WriteTimeout time.Duration // SERVER_WRITE_TIMEOUT
	IdleTimeout  time.Duration // SERVER_IDLE_TIMEOUT
}

type PostgresConfig struct {
	Host     string // PG_HOST
	Port     int    // PG_PORT
	User     string // PG_USER (обязательная)
	Password string // PG_PASSWORD (обязательная)
	Database string // PG_DATABASE (обязательная)
	SSLMode  string // PG_SSLMODE

	MaxConns        int32         // PG_MAX_CONNS
	MinConns        int32         // PG_MIN_CONNS
	MaxConnLifetime time.Duration // PG_MAX_CONN_LIFETIME
	MaxConnIdleTime time.Duration // PG_MAX_CONN_IDLE_TIME
}

// DSN формирует строку подключения к PostgreSQL.
// Используется net/url, чтобы корректно экранировать спецсимволы
// (@, :, /, %, # и т.п.) в логине и пароле — raw fmt.Sprintf привёл бы
// к невалидному или неверно распарсенному URL при их наличии.
func (c PostgresConfig) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     "/" + c.Database,
		RawQuery: url.Values{"sslmode": {c.SSLMode}}.Encode(),
	}
	return u.String()
}

// MustLoad читает конфигурацию. Паникует при отсутствии обязательных переменных или неверном формате.
func MustLoad() Config {
	return Config{
		Server: ServerConfig{
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 5*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		},
		Postgres: PostgresConfig{
			Host:            getEnv("PG_HOST", "localhost"),
			Port:            getEnvInt("PG_PORT", 5432),
			User:            mustGetEnv("PG_USER"),
			Password:        mustGetEnv("PG_PASSWORD"),
			Database:        mustGetEnv("PG_DATABASE"),
			SSLMode:         getEnv("PG_SSLMODE", "disable"),
			MaxConns:        int32(getEnvInt("PG_MAX_CONNS", 25)),
			MinConns:        int32(getEnvInt("PG_MIN_CONNS", 5)),
			MaxConnLifetime: getEnvDuration("PG_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime: getEnvDuration("PG_MAX_CONN_IDLE_TIME", 5*time.Minute),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func mustGetEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
}

func getEnvInt(key string, defaultVal int) int {
	valStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s must be an integer, got: %s", key, valStr))
	}
	return val
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	valStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}
	val, err := time.ParseDuration(valStr)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s must be a valid duration, got: %s", key, valStr))
	}
	return val
}
