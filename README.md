# Rate Limiter Middleware

HTTP middleware для ограничения количества запросов от одного пользователя/IP за период времени. Production-ready: structured logging, health probes, fail-open, Docker non-root.

## Возможности

- **Token Bucket** — алгоритм ограничения с плавным пополнением токенов
- **Redis** — хранение с Lua-скриптом, retry, таймауты
- **429 Too Many Requests** — JSON-ответы с Request-ID
- **Fail-open** — при недоступности Redis пропускает трафик (настраивается)
- **Health probes** — `/live` (liveness), `/ready` (readiness + Redis check)
- **Structured logging** — JSON (slog)
- **Prometheus метрики** — решения, латентность, статусы
- **Graceful shutdown** — SIGINT/SIGTERM, настраиваемый таймаут
- **Request ID** — X-Request-ID для трассировки
- **Recovery** — перехват panic, 500 с логированием

## Быстрый старт

### Локально (нужен Redis)

```bash
docker run -d -p 6379:6379 redis:7-alpine
cp .env.example .env  # опционально
go run ./cmd/server
```

### Docker Compose

```bash
docker compose up --build
```

| Endpoint | Описание |
|----------|----------|
| http://localhost:8080 | API |
| http://localhost:8080/live | Liveness probe (k8s) |
| http://localhost:8080/ready | Readiness probe (проверка Redis) |
| http://localhost:8080/health | Alias для readiness |
| http://localhost:9090/metrics | Prometheus |

## Конфигурация

См. `.env.example`. Основные переменные:

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `HTTP_ADDR` | :8080 | HTTP-сервер |
| `METRICS_ADDR` | :9090 | Prometheus |
| `REDIS_ADDR` | localhost:6379 | Redis |
| `REDIS_TIMEOUT` | 3s | Таймаут операций Redis |
| `REDIS_RETRIES` | 3 | Повторы при ошибках |
| `RATE_LIMIT_CAPACITY` | 100 | Ёмкость bucket |
| `RATE_LIMIT_REFILL` | 1.67 | Токены/сек |
| `RATE_LIMIT_FAIL_OPEN` | true | При ошибке Redis: пропускать трафик |
| `SHUTDOWN_TIMEOUT` | 15s | Таймаут graceful shutdown |
| `LOG_LEVEL` | info | debug, info, warn, error |

## Kubernetes

```yaml
livenessProbe:
  httpGet:
    path: /live
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 2
  periodSeconds: 5
```

## API

Успешный ответ:
```json
{"status":"ok"}
```

Ошибка 429:
```json
{"error":"Too Many Requests","code":429,"request_id":"a1b2c3d4"}
```

Заголовок `X-Request-ID` в ответе для трассировки.

## Benchmark & Load Test

```bash
# Redis должен быть запущен
go test -bench=. -benchmem ./internal/limiter/... ./internal/middleware/...

go run ./scripts/loadtest.go -url http://localhost:8080 -n 1000 -c 50
```

## Lint

```bash
golangci-lint run
```

## Структура проекта

```
rate limiter/
├── cmd/server/main.go
├── internal/
│   ├── api/errors.go         # JSON error responses
│   ├── config/config.go      # Validated config
│   ├── health/health.go      # /live, /ready
│   ├── limiter/
│   ├── logger/logger.go      # Structured logging
│   ├── middleware/
│   │   ├── rate_limit.go
│   │   ├── request_id.go
│   │   └── recovery.go
│   └── metrics/
├── scripts/loadtest.go
├── .env.example
├── .golangci.yml
├── Dockerfile                # non-root, healthcheck
└── docker-compose.yml
```
