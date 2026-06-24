# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ffmpeg ca-certificates

# Run as non-root user for security
RUN addgroup -S comrad && adduser -S comrad -G comrad

WORKDIR /app

COPY --from=builder /server /app/server
COPY migrations/ /app/migrations/
COPY web/ /app/web/

RUN mkdir -p /app/segments && chown -R comrad:comrad /app

USER comrad

EXPOSE 8080 1935

# Health check hits the public health endpoint so orchestrators can detect a
# wedged server. Uses busybox wget (bundled with Alpine).
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- "http://localhost:${HTTP_PORT:-8080}/api/health" >/dev/null 2>&1 || exit 1

CMD ["/app/server"]
