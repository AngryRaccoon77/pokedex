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

   Под Windows/PowerShell переменные из `.env` подхватываются автоматически приложением при старте (через `internal/config`), отдельно `source .env` делать не нужно — главное, чтобы файл `.env` лежал в корне проекта.

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
| `GET` | `/api/v1/pokemon` | список покемонов; query-параметры: `catchable` (bool), `type` (enum), `search` (подстрока по `name`/`description`), `limit`, `offset` |
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
