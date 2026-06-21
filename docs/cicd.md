# CI/CD-пайплайн (GitHub Actions → ghcr.io → ArgoCD → kind)

Самостоятельная работа по курсу инфраструктуры. CI/CD-конвейер: сборка
Go-приложения по git-тегу, публикация образа в GitHub Container Registry,
автоматическая доставка в локальный Kubernetes (kind) через ArgoCD (pull-based GitOps),
доступ по доменному имени через nip.io.

## Что и как закрывает требования задания

| Требование задания | Реализация |
|---|---|
| Приложение обрабатывает HTTP GET | Go + chi, `GET /health`, `GET /api/v1/pokemon`, порт 8085 |
| Код в git-репозитории | Этот репозиторий |
| Сборка запускается при появлении git-тега | Workflow `release.yml`, триггер `on.push.tags: v*.*.*` |
| Авто-доставка в Kubernetes | ArgoCD `Application` с `syncPolicy.automated` следит за `k8s/` |
| Доступ по своему доменному имени | Ingress на `pokedex.127.0.0.1.nip.io` |

## Архитектура

```
 git tag v1.0.0
      │
      ▼
┌──────────────────────────┐
│ GitHub Actions (release) │
│ 1. build app image        │  ──push──►  ghcr.io/<owner>/pokedex-api:1.0.0
│ 2. patch k8s/20-deploy…   │  ──commit──► main
└──────────────────────────┘
      │ (git изменился)
      ▼
┌──────────────────────────┐
│ ArgoCD (в кластере)      │  ──watch──►  k8s/  в этом репо
│ auto-sync + self-heal    │
└──────────────────────────┘
      │ apply
      ▼
┌──────────────────────────┐
│ kind cluster (namespace pokedex)            │
│  Postgres (StatefulSet) ◄── initContainer ждёт готовности
│  pokedex-api (Deployment) ── Service ── Ingress
│  (миграции goose накатывает само приложение при старте)
└──────────────────────────┘
      │
      ▼  http://pokedex.127.0.0.1.nip.io/health
```

Ключевая идея — **pull-based GitOps**: раннер GitHub НЕ имеет сетевого доступа
к локальному кластеру и ничего в него не пушит. Раннер только публикует образ
и правит git. ArgoCD внутри кластера сам забирает изменения. Поэтому кластер
не нужно выставлять наружу.

Отдельного образа/initContainer для миграций нет: `cmd/server/main.go`
сам прогоняет `goose` по встроенным (`embed.FS`) `.sql`-файлам из `migrations/`
перед тем, как поднять HTTP-сервер — один образ, один тег = код + схема БД.

## Структура репозитория

```
.
├── cmd/server/                  # точка входа приложения
├── internal/                    # домены, хендлеры, репозитории
├── migrations/                  # .sql-миграции goose (встроены в бинарь через embed.FS)
├── Dockerfile                   # сборка Go-приложения
├── kind-config.yaml             # kind с пробросом 80/443
├── k8s/                         # ← за этим каталогом следит ArgoCD
│   ├── 00-namespace-secret.yaml
│   ├── 10-postgres.yaml
│   ├── 20-deployment.yaml       # ← CI патчит тег образа здесь
│   ├── 30-service.yaml
│   └── 40-ingress.yaml
├── argocd/
│   └── application.yaml         # ArgoCD Application (применяется один раз руками)
└── .github/workflows/
    └── release.yml              # CI: сборка по тегу + bump манифеста
```

## Подготовка

Владелец и репозиторий уже подставлены: `angryraccoon77` (lowercase, как
требует ghcr.io) в `k8s/20-deployment.yaml`, и
`https://github.com/AngryRaccoon77/pokedex.git` в `argocd/application.yaml`.
Если репозиторий переименуется или сменится владелец — поправить обе строки.

Переменные окружения приложения в `k8s/20-deployment.yaml` уже соответствуют
тому, что читает `internal/config` (`PG_HOST`, `PG_PORT`, `PG_USER`,
`PG_PASSWORD`, `PG_DATABASE`, `PG_SSLMODE`, `SERVER_PORT`) — менять не нужно,
если не меняли сам `config.go`.

## Запуск стенда (один раз)

### 0. Зависимости
Нужны: `docker`, `kind`, `kubectl`, `helm` (опционально).

### 1. Создать кластер kind с пробросом портов
```bash
kind create cluster --name pokedex --config kind-config.yaml
```

### 2. Установить ingress-nginx (kind-вариант)
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# дождаться готовности контроллера
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s
```
> ⚠️ Важно для защиты: в ноябре 2025 анонсировано прекращение поддержки
> ingress-nginx в марте 2026. Для учебного стенда он подходит (kind-манифест
> «просто работает»), но если спросят про продакшн — упомяни миграцию на
> Gateway API или Traefik как осознанный план.

### 3. Установить ArgoCD
```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

kubectl wait --namespace argocd \
  --for=condition=available deployment/argocd-server \
  --timeout=180s

# пароль admin для UI
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d; echo
```
UI (по желанию): `kubectl -n argocd port-forward svc/argocd-server 8080:443`
→ https://localhost:8080 (login: `admin`).

### 4. Дать ArgoCD приватный доступ к репозиторию (если репо приватное)
Если репо публичное — пропусти. Если приватное:
```bash
argocd login localhost:8080 --username admin --password <пароль> --insecure
argocd repo add https://github.com/AngryRaccoon77/pokedex.git \
  --username AngryRaccoon77 --password <GitHub PAT с доступом repo>
```

### 5. Применить ArgoCD Application
```bash
kubectl apply -f argocd/application.yaml
```
С этого момента ArgoCD сам подтянет всё из `k8s/` и будет держать кластер
в синхроне с git.

## Рабочий цикл (то, что демонстрируешь на защите)

```bash
# что-то поменяли в коде...
git add .
git commit -m "feat: новый эндпоинт"
git push

# выпускаем релиз
git tag v1.0.0
git push origin v1.0.0
```

Дальше автоматически:
1. Workflow `release` собирает `pokedex-api:1.0.0`, пушит в ghcr.io.
2. Тот же workflow патчит тег образа в `k8s/20-deployment.yaml` и коммитит
   в `main` с пометкой `[skip ci]`.
3. ArgoCD замечает изменение в git и синкает Deployment в кластер
   (обычно в пределах ~3 минут; можно форсировать `argocd app sync pokedex`).
4. Под приложения стартует, сам прогоняет `goose up` по встроенным
   миграциям и поднимает HTTP-сервер.

Проверка:
```bash
curl http://pokedex.127.0.0.1.nip.io/health
curl http://pokedex.127.0.0.1.nip.io/api/v1/pokemon
```

## Первый деплой: загвоздка курицы и яйца

В `k8s/20-deployment.yaml` тег образа изначально стоит как `:latest` с
плейсхолдером владельца. Первый реальный образ появится только после первого
тега. Поэтому правильный порядок:
1. Запушить `main` в `origin`.
2. Выпустить первый тег `v0.1.0` — CI соберёт образ и пропатчит манифест на
   `:0.1.0`.
3. Только после этого применять `argocd/application.yaml` (или, если применил
   раньше, ArgoCD сам подхватит исправленный манифест после патча).

## Полезные команды для отладки

```bash
# поды и события namespace приложения
kubectl -n pokedex get pods
kubectl -n pokedex describe pod -l app=pokedex-api

# логи приложения (включая старт миграций)
kubectl -n pokedex logs deploy/pokedex-api -c pokedex-api

# статус ArgoCD-приложения
argocd app get pokedex
kubectl -n argocd get applications

# проверить, что образ реально опубликован
# (Packages на странице профиля GitHub)
```

## Очистка
```bash
kind delete cluster --name pokedex
```

## Возможные вопросы на защите (и честные ответы)

- **Почему ArgoCD, а не `kubectl apply` из Actions?**
  Локальный кластер недоступен раннеру по сети. Pull-модель снимает
  необходимость открывать кластер наружу и хранить kubeconfig в секретах CI.

- **Почему миграции не в отдельном образе/initContainer?**
  Приложение само встраивает `.sql`-файлы через `embed.FS` и прогоняет
  `goose up` перед запуском HTTP-сервера (`cmd/server/main.go`). Один тег —
  один артефакт «код + миграции», без риска разъехаться по версиям.

- **Как это не зацикливается, ведь CI коммитит в main?**
  Workflow триггерится только по тегам `v*.*.*`, а не по push в ветку.
  Коммит-патч идёт в `main` и нового запуска не вызывает (плюс `[skip ci]`).

- **Что если миграция упадёт?**
  Приложение завершится с ошибкой и `os.Exit(1)` до старта сервера, под не
  перейдёт в Ready, старый под (при наличии) продолжит обслуживать трафик —
  деплой фактически не состоится, это ожидаемое поведение.
