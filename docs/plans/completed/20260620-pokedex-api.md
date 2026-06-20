# Pokédex API — курсовая работа (chi + pgx + goose)

## Overview
- Микросервис на Go, реализующий REST CRUD над сущностью `pokemon` (Покедекс).
- Делается как курсовая по микросервисам: показать связку **chi (роутинг) + pgx (доступ к БД) + goose (миграции)** на собственной предметной области.
- Берётся за основу рабочий образец `D:\go\Код\Примеры для лаб\go-chi-pgx-api` (сущность `items`) и переделывается под `pokemon`. Намеренные отличия от образца: (1) переименование сущности `items`→`pokemon`; (2) смена типа числового поля `price float`→`power int`; (3) новое enum-поле `type` с дополнительной осью фильтрации `?type=`; (4) поиск `search` по `name` И `description` (в образце — только по `title`).
- Целевой проект — новая независимая папка `D:\go\Код\pokedex-api` (модуль `pokedex-api`), образец не трогаем.

## Context (from discovery)
Образец — слоистый микросервис, структура переносится 1:1:
- `cmd/server/main.go` — сборка зависимостей снизу вверх, goose-миграции через pgx stdlib-адаптер, `pgxpool`, chi-роутер с middleware (RequestID, RealIP, structured logger, Recoverer, Timeout, CORS), `/health`, graceful shutdown по SIGINT/SIGTERM.
- `internal/domain` — бизнес-модели, input-структуры с `Validate()`, доменные ошибки (`errors.New`), без зависимостей от HTTP/БД.
- `internal/repository` — интерфейс `*Repository` (per-entity, не generic) + реализация на `pgxpool`; динамический `WHERE` через `$N`-плейсхолдеры; маппинг `pgconn.PgError` код `23505` → доменная ошибка дубликата; `pgx.CollectRows` + `RowToStructByPos`.
- `internal/service` — тонкая бизнес-логика: `Validate()` → вызов репозитория → логирование; оборачивает ошибки через `fmt.Errorf("...: %w", err)`.
- `internal/handlers` — разбор HTTP, маппинг доменных ошибок на статусы, `respondJSON`/`respondError`.
- `internal/middleware/logging.go` — structured slog-логгер запросов.
- `internal/config/config.go` — конфиг из env с `MustLoad()`.
- `migrations/` — `*.sql` (goose) + `migrations.go` с `//go:embed *.sql`.
- Корень: `go.mod`, `.env` / `.env.example`, `api.http`, `docker-compose.yml`.

Замеченные паттерны (соблюдаем):
- Указатели в `Update*Input` (`*string`/`*int`/`*bool`) — отличить «не передано» от «обнулить»; в SQL `COALESCE($N, col)`.
- Update в транзакции с проверкой существования (`SELECT EXISTS`).
- `RowToStructByPos` требует, чтобы **порядок полей структуры совпадал с порядком колонок в SELECT**.
- Стиль пользователя: идиоматичный Go, тонкий handler → service → узкий store, без обобщённого Repository (per-entity интерфейс из образца допустим — это не generic).

Зависимости (`go.mod`, go 1.26.x): `go-chi/chi/v5`, `go-chi/cors`, `jackc/pgx/v5`, `pressly/goose/v3`.

## Development Approach
- **Testing approach: Regular** (код, затем тесты) — соответствует образцу (в нём тестов нет) и предметной области курсача.
- **Реалистичная стратегия тестов для этого проекта:**
  - **Автоматические unit-тесты — на доменном слое** (`domain`): `Validate()` у Create/Update-инпутов и валидация `type`. Это чистые функции без БД/HTTP — тестируются тривиально и дают настоящее покрытие логики.
  - **HTTP + pgx слои проверяются вручную через `api.http`** (как и задумано в образце — там нет автотестов, а repository требует живой Postgres). Это осознанный выбор, а не пропуск тестов: интеграционные тесты с поднятием БД выходят за объём курсача.
- Каждую задачу доводим до конца перед переходом к следующей; после доменных задач — гоняем `go test ./...`.
- `go build ./...` должен проходить после каждой задачи.

## Testing Strategy
- **Unit-тесты (`go test ./...`)**: `internal/domain/pokemon_test.go` — таблично-управляемые тесты на `CreatePokemonInput.Validate`, `UpdatePokemonInput.Validate`, `IsValidType` (успех + все ветки ошибок: пустое имя, отрицательная сила, невалидный тип).
- **Ручная проверка (`api.http`)**: health, list (+фильтры `catchable`/`type`/`search`), create (валидный), get by id, partial update, и 4 ошибочных сценария (нет имени → 400, отрицательная сила → 400, невалидный тип → 400, дубликат имени → 409), delete, get удалённого → 404.
- **Smoke-проверка**: `docker compose up -d` → `go run ./cmd/server` → прогон `api.http` сверху вниз.

## Progress Tracking
- `[x]` — выполнено; ➕ —新ая задача; ⚠️ — блокер.
- Держать план в синхроне с фактической реализацией.

## Solution Overview
- **Архитектура** — без изменений против образца: handler (HTTP) → service (бизнес-логика/валидация) → repository (pgx). Domain не зависит ни от чего.
- **Ключевые доменные решения (темы для защиты):**
  1. **`price float64` → `power int`.** Статы покемонов целочисленные; `DOUBLE PRECISION` не нужен. Меняется: тип колонки в миграции (`INTEGER`), поле `Power int` в домене, тип в Create/Update-инпутах (`int` / `*int`), валидация `power < 0`. *Защита: соответствие предметной области, отсутствие плавающей точки там, где она бессмысленна.*
  2. **Валидация в доменном слое, а не в хендлере.** `Validate()` живёт на input-структурах в `domain`; хендлер только декодирует JSON и маппит ошибки на коды. *Защита: бизнес-правила не зависят от транспорта; их можно переиспользовать и юнит-тестировать без HTTP.*
  3. **Маппинг UNIQUE-конфликта в репозитории.** `name` — `UNIQUE`; нарушение ловится как `pgconn.PgError` с кодом `23505` и превращается в `domain.ErrDuplicateName`. *Защита: стор не «протекает» наружу деталями БД — сервис/хендлер видят доменную ошибку, а не pg-код.*
  4. **Новое поле `type` (enum-строка).** Допустимый набор хранится в домене (`map[string]struct{}`), валидируется в `Validate()`. Даёт вторую ось фильтра (`?type=fire`) и ошибку `ErrInvalidType` (400).

## Technical Details

### Сущность и схема
Таблица `pokemon`:
| колонка | тип | примечание |
|---|---|---|
| `id` | `BIGSERIAL PRIMARY KEY` | |
| `name` | `TEXT NOT NULL UNIQUE` | бывш. `title` |
| `description` | `TEXT NOT NULL DEFAULT ''` | |
| `power` | `INTEGER NOT NULL DEFAULT 0` | бывш. `price`, тип сменён |
| `type` | `TEXT NOT NULL` | новое, enum-строка |
| `catchable` | `BOOLEAN NOT NULL DEFAULT true` | бывш. `active` |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | |
| `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | |

Индексы: `idx_pokemon_catchable(catchable)`, `idx_pokemon_created_at(created_at DESC)`, `idx_pokemon_type(type)`.

### Доменные структуры
- `Pokemon{ ID, Name, Description, Power int, Type, Catchable bool, CreatedAt, UpdatedAt }` — порядок полей = порядок колонок в SELECT (для `RowToStructByPos`).
- `CreatePokemonInput{ Name, Description, Power int, Type }` + `Validate()`.
- `UpdatePokemonInput{ Name *string, Description *string, Power *int, Type *string, Catchable *bool }` + `Validate()`.
- `PokemonFilter{ CatchableOnly bool, Type string, Search string, Limit, Offset int }`.
- Допустимые типы: `fire, water, grass, electric, psychic, normal, ice, dragon, dark, fairy, fighting, poison, ground, flying, bug, rock, ghost, steel` (множество в `domain`), функция `IsValidType(string) bool`.

### Доменные ошибки → HTTP
| ошибка | код |
|---|---|
| `ErrNotFound` | 404 |
| `ErrDuplicateName` | 409 |
| `ErrNameRequired` | 400 |
| `ErrNegativePower` | 400 |
| `ErrInvalidType` | 400 |

### API (chi), монтируется под `/api/v1/pokemon`
- `GET /pokemon?catchable=true&type=fire&search=char&limit=20&offset=0`
- `GET /pokemon/{id}`
- `POST /pokemon`
- `PUT /pokemon/{id}`
- `DELETE /pokemon/{id}`

### Processing flow
HTTP → handler (decode + map errors) → service (`Validate()` + log) → repository (pgx: `QueryRow`/`Query`, динамический WHERE, `COALESCE` в Update, маппинг `23505`).

## What Goes Where
- **Implementation Steps** (`[ ]`): создание нового проекта, все Go-файлы, миграция, домен-тесты, обвязка (docker-compose/.env/api.http), сборка и прогон.
- **Post-Completion** (без чекбоксов): ручной smoke через `api.http`, написание текстовой части отчёта с ответами на вопросы к защите.

## Implementation Steps

### Task 1: Скелет проекта и зависимости

**Files:**
- Create: `D:/go/Код/pokedex-api/go.mod`
- Create: `D:/go/Код/pokedex-api/.gitignore`
- Create: `D:/go/Код/pokedex-api/migrations/migrations.go`

- [x] создать папку `D:/go/Код/pokedex-api`, в ней `go mod init pokedex-api` (go-директива `1.26.3` — как в образце)
- [x] добавить зависимости: `go get github.com/go-chi/chi/v5 github.com/go-chi/cors github.com/jackc/pgx/v5 github.com/pressly/goose/v3`
- [x] создать `migrations/migrations.go` с `//go:embed *.sql` и `var EmbedFS embed.FS` (копия образца)
- [x] `.gitignore` (`.env`, бинарники)
- [x] `go build ./...` (пустой — должен пройти после появления пакетов; на этом шаге может быть «no Go files» — ок) — на деле упало с ошибкой `pattern *.sql: no matching files found`, т.к. SQL-миграция появится только в Task 2; go.mod/go.sum собраны корректно, зависимости подключены

### Task 2: Миграция goose для таблицы pokemon

**Files:**
- Create: `D:/go/Код/pokedex-api/migrations/20260620000000_create_pokemon_table.sql`

- [x] `-- +goose Up`: `CREATE TABLE IF NOT EXISTS pokemon` со всеми колонками из Technical Details (`power INTEGER`, `type TEXT NOT NULL`, `catchable BOOLEAN`)
- [x] создать индексы `idx_pokemon_catchable`, `idx_pokemon_created_at`, `idx_pokemon_type`
- [x] `-- +goose Down`: `DROP TABLE IF EXISTS pokemon`
- [x] структура goose как в образце: **один** `StatementBegin/StatementEnd` вокруг всего Up (CREATE TABLE + 3 индекса), отдельная пара вокруг Down
- [x] визуально сверить порядок и типы колонок со структурой `Pokemon` (для `RowToStructByPos`)

### Task 3: Доменный слой (модель, инпуты, ошибки, валидация типов)

**Files:**
- Create: `D:/go/Код/pokedex-api/internal/domain/pokemon.go`
- Create: `D:/go/Код/pokedex-api/internal/domain/pokemon_test.go`

- [x] объявить `Pokemon`, `CreatePokemonInput`, `UpdatePokemonInput`, `PokemonFilter` (порядок полей `Pokemon` = порядок SELECT-колонок)
- [x] объявить множество допустимых типов и `IsValidType(string) bool`
- [x] объявить ошибки: `ErrNotFound, ErrDuplicateName, ErrNameRequired, ErrNegativePower, ErrInvalidType`
- [x] реализовать `CreatePokemonInput.Validate()` (имя непустое; `Power >= 0`; `IsValidType(Type)`)
- [x] реализовать `UpdatePokemonInput.Validate()` (если `Power != nil` → `>=0`; если `Type != nil` → `IsValidType`)
- [x] написать таблично-управляемые тесты на `Validate()` обоих инпутов и `IsValidType` (успех + ветки `ErrNameRequired`/`ErrNegativePower`/`ErrInvalidType`)
- [x] `go test ./internal/domain/...` — должны пройти до следующей задачи

### Task 4: Repository (pgx) — интерфейс и реализация

**Files:**
- Create: `D:/go/Код/pokedex-api/internal/repository/repository.go`
- Create: `D:/go/Код/pokedex-api/internal/repository/postgres.go`

- [x] `repository.go`: интерфейс `PokemonRepository` (GetByID/List/Create/Update/Delete) — per-entity, не generic
- [x] `postgres.go`: `postgresPokemonRepo{pool *pgxpool.Pool}` + конструктор `NewPostgresPokemonRepo`
- [x] `GetByID` / `List` через `pgx.CollectRows` + `RowToStructByPos[domain.Pokemon]`; `pgx.ErrNoRows` → `ErrNotFound`
- [x] `List`: динамический WHERE с ветками `CatchableOnly`, `Type` (новая, пропускать при пустой строке), `Search` (ILIKE по `name` И `description` — осознанное расширение против образца, где поиск только по `title`; см. Overview), `ORDER BY created_at DESC`, LIMIT/OFFSET
- [x] `Create`: **обязательно включить `type` в список колонок INSERT** — `INSERT INTO pokemon (name, description, power, type) VALUES ($1,$2,$3,$4) RETURNING ...` с `input.Type` параметром. ⚠️ `type` объявлен `NOT NULL` без DEFAULT — если скопировать образец 1:1 (там INSERT без всех колонок), все вставки упадут на NOT NULL violation.
- [x] `Update`: в транзакции с `SELECT EXISTS` и `COALESCE($N, col)` по полям (включая `type`, `catchable`); `Delete` через `Exec` + `RowsAffected()==0 → ErrNotFound`
- [x] маппинг `pgconn.PgError` код `23505` → `domain.ErrDuplicateName` в Create и Update
- [x] *(тесты репозитория не автоматизируем — требуют живой БД; проверка в Task 9 через `api.http`)*
- [x] `go build ./...`

### Task 5: Service слой

**Files:**
- Create: `D:/go/Код/pokedex-api/internal/service/pokemon.go`

- [x] `PokemonService{repo PokemonRepository, logger *slog.Logger}` + конструктор
- [x] методы `GetByID/List/Create/Update/Delete`: в Create/Update вызвать `input.Validate()` до репозитория
- [x] логировать события (`pokemon created/updated/deleted`) с `slog`
- [x] оборачивать ошибки репозитория через `fmt.Errorf("service ...: %w", err)`
- [x] `go build ./...`

### Task 6: HTTP handlers + роутинг сущности

**Files:**
- Create: `D:/go/Код/pokedex-api/internal/handlers/pokemon.go`

- [x] `PokemonHandler{svc, logger}` + `Routes()` (`GET /`, `POST /`, `GET/PUT/DELETE /{id}`)
- [x] `List`: собрать `PokemonFilter` из query (`catchable`, `type`, `search`, `limit`, `offset`)
- [x] `Get/Create/Update/Delete`: decode JSON, вызвать сервис, вернуть статус
- [x] `handleError`: маппинг 5 доменных ошибок на 404/409/400; default → 500 + лог
- [x] хелперы `respondJSON/respondError/parseID/queryInt` (перенести из образца)
- [x] `go build ./...` — также обнаружено и исправлено: `go-chi/chi/v5` и `go-chi/cors` не были фактически добавлены в go.mod на Task 1, выполнен `go get` для обоих перед сборкой

### Task 7: Config, middleware, main (сборка приложения)

**Files:**
- Create: `D:/go/Код/pokedex-api/internal/config/config.go`
- Create: `D:/go/Код/pokedex-api/internal/middleware/logging.go`
- Create: `D:/go/Код/pokedex-api/cmd/server/main.go`

- [x] `config.go` — перенести из образца как есть (env, `MustLoad`, `DSN`)
- [x] `middleware/logging.go` — перенести из образца как есть
- [x] `main.go` — перенести из образца, заменив сборку: `NewPostgresPokemonRepo` → `NewPokemonService` → `NewPokemonHandler`; маршрут `r.Mount("/pokemon", pokemonHandler.Routes())`; импорт `pokedex-api/...`
- [x] оставить goose-миграции, pgxpool, middleware-цепочку, `/health`, graceful shutdown без изменений
- [x] `go build ./...` — приложение должно собираться — также обнаружено и исправлено: `github.com/pressly/goose/v3` не был добавлен в go.mod на Task 1, выполнен `go get` перед сборкой

### Task 8: Обвязка проекта (env, docker-compose, api.http)

**Files:**
- Create: `D:/go/Код/pokedex-api/.env.example`
- Create: `D:/go/Код/pokedex-api/.env`
- Create: `D:/go/Код/pokedex-api/docker-compose.yml`
- Create: `D:/go/Код/pokedex-api/api.http`

- [x] `.env.example` / `.env` — на базе образца; `PG_DATABASE=pokedex` (порт сервера на выбор, напр. 8085) — выбран порт `8085`
- [x] `docker-compose.yml` — postgres:17-alpine, `POSTGRES_DB: pokedex`, healthcheck под `pokedex` — проверено `docker compose config`
- [x] `api.http` — переписать сценарии под `pokemon`: health, list (+`?type=fire`, `?catchable=true`, `?search=`), create (валидный, напр. Pikachu/electric — в ответе проверить, что `type` вернулся непустым), get by id, partial update (сменить `power`/`catchable`), 4 ошибки (нет имени, отрицательная сила, невалидный `type`, дубликат имени), delete, get → 404
- [x] проверить, что `@baseUrl` совпадает с `SERVER_PORT` — оба `8085`

### Task 9: Verify acceptance criteria
- [x] `go vet ./...` и `go build ./...` — без ошибок (прошли с первого раза)
- [x] `go test ./...` — доменные тесты зелёные (`ok pokedex-api/internal/domain 0.566s`)
- [x] `docker compose up -d` → `go run ./cmd/server` стартует, миграция применяется, `/health` отвечает `{"status":"ok"}` — порт 5432 на хосте оказался занят сторонним контейнером `smarteagle-postgres`; перемаппили `docker-compose.yml`/`.env`/`.env.example` на хост-порт `5433` (контейнер слушает 5432 внутри), после чего compose поднялся и healthcheck прошёл; миграция `20260620000000_create_pokemon_table.sql` применилась автоматически при старте сервера
- [x] прогнать `api.http` сверху вниз: CRUD работает, фильтры `type`/`catchable`/`search` работают, все 4 ошибки дают ожидаемые коды (400/400/400/409), удаление → 404 — все 14 сценариев прошли успешно без изменений в Go-коде (domain/repository/service/handlers не потребовали правок)
- [x] сверить, что все требования из Overview реализованы — `power int`, `type` enum-фильтр, `search` по name+description, маппинг 23505→409 — все подтверждены живыми запросами

### Task 10: [Final] Документация и финализация
- [x] создать краткий `README.md` в `pokedex-api` (как запустить: compose + run, список эндпоинтов)
- [x] обновить память проекта при необходимости (статус курсача) (skipped here — orchestrator will update memory after this task)
- [x] переместить этот план в `docs/plans/completed/`

## Post-Completion
*Без чекбоксов — ручные/внешние действия.*

**Ручная проверка:**
- Полный прогон `api.http` против локального стенда (Postgres в docker).
- Проверка edge-кейсов фильтрации (комбинация `type` + `catchable` + `search` + пагинация).

**Отчёт (курсовая):**
- Текстовая часть с архитектурой (chi + pgx + goose, слои) и **ответами на вопросы к защите**:
  1. почему `price float` → `power int`;
  2. почему валидация в доменном слое, а не в хендлере;
  3. как репозиторий маппит UNIQUE-конфликт (`pgconn.PgError` `23505` → `ErrDuplicateName`), не «протекая» деталями БД;
  4. зачем указатели в `UpdatePokemonInput` и `COALESCE` в SQL.
- Скриншоты запросов/ответов из `api.http` для иллюстраций.
