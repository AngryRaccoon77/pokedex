# ---- build stage ----
FROM golang:1.26-alpine AS build

WORKDIR /src

# Кэшируем зависимости отдельным слоем
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Статический бинарь без CGO, чтобы спокойно жить в alpine
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/pokedex-api \
    ./cmd/server

# ---- runtime stage ----
FROM alpine:3.20

# ca-certificates на случай исходящих HTTPS-вызовов, wget для healthcheck/отладки
RUN apk add --no-cache ca-certificates wget && \
    addgroup -S app && adduser -S app -G app

COPY --from=build /out/pokedex-api /usr/local/bin/pokedex-api

USER app
EXPOSE 8085

ENTRYPOINT ["/usr/local/bin/pokedex-api"]
