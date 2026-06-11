# Образ onebase для нагрузочного стенда. Контекст сборки — КОРЕНЬ репозитория:
#   docker build -f loadtest/Dockerfile -t onebase:loadtest .
# (docker-compose в этой папке уже задаёт context: .. — см. docker-compose.yml)

FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO выключен (как в build.sh) — SQLite-драйвер modernc чистый Go, статический бинарь.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/onebase ./cmd/onebase

FROM alpine:3.20
RUN adduser -D -u 10001 onebase
WORKDIR /app
# Бинарь + эталонная конфигурация для нагрузки.
COPY --from=build /out/onebase /usr/local/bin/onebase
COPY --from=build /src/examples /app/examples
USER onebase
EXPOSE 8080
# Профилирование/метрики включаются токеном (см. internal/api/server.go).
ENV ONEBASE_DEBUG_TOKEN=loadtest
# Конкретную команду (migrate + run) задаёт docker-compose.
ENTRYPOINT ["onebase"]
