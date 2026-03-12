# ===== Build Stage =====
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/server/

# ===== Run Stage =====
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
COPY config/ ./config/

# Port is set via FLASH_SALE_SERVER_PORT env var at runtime
CMD ["./server"]
