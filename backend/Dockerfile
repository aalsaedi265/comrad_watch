# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ffmpeg ca-certificates

WORKDIR /app

COPY --from=builder /server /app/server
COPY migrations/ /app/migrations/

RUN mkdir -p /app/segments

EXPOSE 8080 1935

CMD ["/app/server"]
