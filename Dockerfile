# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source tree
COPY . .

# Build application server, seed, and reset utilities
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/seed ./cmd/seed
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/reset ./cmd/reset

# Production stage
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl

# Copy compiled binaries
COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/bin/seed /app/seed
COPY --from=builder /app/bin/reset /app/reset

# Copy runtime assets
COPY web /app/web
COPY migrations /app/migrations
COPY scripts/fixtures /app/scripts/fixtures
COPY config /app/config
COPY docs /app/docs

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --retries=5 CMD curl -f http://localhost:8080/healthz || exit 1

CMD ["/app/server"]

