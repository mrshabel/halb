# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Build the application
COPY cmd/ ./cmd/
COPY halb/ ./halb/
COPY configs/ ./configs/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o halb ./cmd/main.go

# Production stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Create non-root user for security
RUN adduser -D -g '' appuser

# Copy binary from builder
COPY --from=builder /app/halb .

# Copy default config
COPY --from=builder /app/configs/config.yaml .

USER appuser

EXPOSE 8000

ENTRYPOINT ["./halb"]
CMD ["-config", "config.yaml"]
