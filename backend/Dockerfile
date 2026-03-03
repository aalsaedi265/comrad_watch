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

CMD ["/app/server"]
