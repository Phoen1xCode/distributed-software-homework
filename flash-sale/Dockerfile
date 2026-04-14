# Build Stage
FROM golang:1.26-alpine AS builder
WORKDIR /app

ARG SERVICE=order-service

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o service ./cmd/${SERVICE}/

# Run Stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/service .
COPY config/ ./config/

CMD ["./service"]
