# Build stage (версия совпадает с директивой go в go.mod)
FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# Run stage - minimal, non-root
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata wget
RUN adduser -D -g '' appuser

WORKDIR /app
COPY --from=builder /server .

USER appuser

EXPOSE 8080 9090

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/ready || exit 1

ENTRYPOINT ["./server"]
