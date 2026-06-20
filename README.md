# Pokedex API

Pokédex REST API — курсовая работа на стеке **chi (роутинг) + pgx (доступ к БД) + goose (миграции)**.

Сервис реализует CRUD над сущностью `pokemon`: поле `power int` (целочисленный стат), enum-поле `type` (тип покемона) с отдельным фильтром, поиск `search` по `name` и `description`.

## Стек

- Go 1.26
- [go-chi/chi](https://github.com/go-chi/chi) — роутинг
- [go-chi/cors](https://github.com/go-chi/cors) — CORS middleware
- [jackc/pgx/v5](https://github.com/jackc/pgx) — драйвер PostgreSQL
- [pressly/goose/v3](https://github.com/pressly/goose) — миграции БД

## Архитектура

```
HTTP → handlers → service → repository (pgx) → PostgreSQL
                       ↑
                    domain (модели, Validate(), доменные ошибки)
```

`internal/domain` не зависит ни от HTTP, ни от БД — валидация бизнес-правил живёт там и покрыта unit-тестами.

## Как запустить

1. Поднять PostgreSQL в Docker:

   ```bash
   docker compose up -d
   ```

2. Задать переменные окружения — взять за основу `.env.example`, скопировать в `.env` и при необходимости поправить (например, порт Postgres):

   ```bash
   cp .env.example .env
   ```

   `internal/config` читает переменные окружения напрямую (`os.LookupEnv`/`os.Getenv`) — `.env` **не подхватывается автоматически**, файла-загрузчика (`godotenv` и т.п.) в проекте нет. Перед запуском нужно явно экспортировать переменные в окружение процесса, например:

   ```bash
   set -a; source .env; set +a   # bash/zsh
   ```

   ```powershell
   Get-Content .env | ForEach-Object {
     if ($_ -match '^\s*([^#=]+)=(.*)$') {
       [System.Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim())
     }
   }   # PowerShell
   ```

   Либо запускать через `docker compose run`/любой раннер, который сам читает `.env` в окружение контейнера. Обязательные переменные без значений по умолчанию — `PG_USER`, `PG_PASSWORD`, `PG_DATABASE`; если хотя бы одна не задана, `config.MustLoad()` паникует при старте. Остальные переменные (`SERVER_PORT`, `PG_HOST`, `PG_PORT`, `PG_SSLMODE`, таймауты и т.д.) имеют значения по умолчанию в `config.go`.

   Порт Postgres на хосте по умолчанию — `5433`, а не стандартный `5432`: во время разработки `5432` был занят другим локальным контейнером Postgres. Если у вас порт `5432` свободен, можно поменять маппинг обратно в `docker-compose.yml`/`.env`/`.env.example`.

3. Запустить сервер:

   ```bash
   go run ./cmd/server
   ```

   При старте автоматически применяются миграции (`goose`) к указанной в `.env` базе. После запуска доступен `GET /health`.

## Эндпоинты

Базовый префикс: `/api/v1/pokemon`.

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/health` | проверка живости сервиса |
| `GET` | `/api/v1/pokemon` | список покемонов; query-параметры: `catchable` (bool), `type` (enum), `search` (подстрока по `name`/`description`), `limit` (по умолчанию 20, максимум 100), `offset` (по умолчанию 0) |
| `GET` | `/api/v1/pokemon/{id}` | получить покемона по ID |
| `POST` | `/api/v1/pokemon` | создать покемона |
| `PUT` | `/api/v1/pokemon/{id}` | частично обновить покемона |
| `DELETE` | `/api/v1/pokemon/{id}` | удалить покемона |

### Коды ошибок

| Ошибка | HTTP-код |
|---|---|
| покемон не найден | 404 |
| дубликат `name` | 409 |
| пустое `name` | 400 |
| отрицательное `power` | 400 |
| невалидный `type` | 400 |

## Тестирование

- **Ручная проверка API**: сценарии в [`api.http`](./api.http) (health, list с фильтрами, create/get/update/delete, ошибочные кейсы) — можно прогнать через REST Client в VS Code/JetBrains HTTP Client.
- **Unit-тесты доменного слоя**:

  ```bash
  go test ./...
  ```

  Покрывают `Validate()` у `CreatePokemonInput`/`UpdatePokemonInput` и `IsValidType`.
