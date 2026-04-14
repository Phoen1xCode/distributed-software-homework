# Flash Sale System

Go backend for a flash-sale e-commerce platform.

## Commands

```bash
make dev/up          # Start PostgreSQL (port 2077)
make dev/down        # Stop PostgreSQL
make dev/reset       # Reset DB (drops volumes)
go run cmd/server/main.go   # Run server (port 8080)
go test ./...        # Run all tests
go test -v -cover ./internal/user/...  # Test specific package
```

## Architecture

Layered architecture: Handler → CachedProductService → ProductService → Repository → GORM/PostgreSQL

```
cmd/server/main.go          # Entry point, wiring
internal/
  middleware/auth.go         # JWT auth + admin check
  user/                      # Auth, registration, profiles
  product/                   # Catalog CRUD, status mgmt, Redis cache (Decorator pattern)
  inventory/                 # Stock with optimistic locking
  order/                     # Orders with inventory saga (bypasses cache intentionally)
pkg/
  cache/                     # Redis client wrapper
  config/                    # Viper YAML + env override
  database/                  # GORM PostgreSQL connection
  response/                  # Standard JSON envelope
config/config.yaml           # App configuration
docker-compose.dev.yaml      # Dev PostgreSQL + Redis
```

## API

Base path: `/api/v1`. Routes: public (no auth), auth (JWT), admin (JWT + role==1).

Health check: `GET /health`

## Key Patterns

- **Optimistic locking**: Inventory uses `version` field in WHERE clause to prevent lost updates
- **Order saga**: Create order = check product → deduct inventory → create order → rollback on failure
- **Config precedence**: YAML < env vars (prefix `FLASH_SALE_`, e.g. `FLASH_SALE_SERVER_PORT`)
- **Response envelope**: All responses use `{code, message, data}` format
- **Pagination**: `{items, total, page, page_size}`, max 100 per page
- **Redis Cache-Aside**: Product detail cached with penetration (null-value), breakdown (singleflight), avalanche (random TTL) protection
- **Decorator pattern**: `CachedProductService` wraps `ProductService` via `ProductServicer` interface
- **Graceful degradation**: Redis failures are non-fatal, falls back to DB

## Gotchas

- JWT claims are `float64` — must cast to `uint` for user_id/role
- Product status: 0=off sale, 1=on sale. Order status: 0=pending, 1=awaiting_payment, 2=paid, 3=cancelled
- DB port is 2077 (not default 5432)
- Redis caches product detail only (`product:detail:{id}`), order module bypasses cache for data freshness
- Passwords excluded from JSON via `json:"-"` tag
- DB auto-migrates all models on startup (User, Product, Inventory, Order)

## Microservice Architecture (HW5)

Three independent services, each with its own database:

```
cmd/order-service/main.go       # Order + Seckill + Product + User (port 8081)
cmd/inventory-service/main.go   # Inventory management (port 8082)
cmd/payment-service/main.go     # Simulated payment (port 8083)
```

### Distributed Transaction: Event-Driven Saga with Transactional Outbox

- Business operation + outbox event written in same DB transaction (atomic)
- Background outbox relay polls unsent events and publishes to Kafka
- Guarantees at-least-once delivery without two-phase commit
- Event types defined in `pkg/event/types.go`
- Outbox model and relay in `pkg/outbox/`

### Saga Flow (Seckill)

1. Order Service: Redis pre-deduct -> create order + ORDER_CREATED outbox event
2. Inventory Service: consumes ORDER_CREATED -> DB deduct -> STOCK_DEDUCTED event
3. Order Service: consumes STOCK_DEDUCTED -> update to AWAITING_PAYMENT -> PAYMENT_REQUESTED event
4. Payment Service: consumes PAYMENT_REQUESTED -> simulate payment -> PAYMENT_SUCCESS event
5. Order Service: consumes PAYMENT_SUCCESS -> update to PAID -> ORDER_COMPLETED event
6. Inventory Service: consumes ORDER_COMPLETED -> confirm deduction (locked -> sold)

### Compensation: STOCK_DEDUCT_FAILED or PAYMENT_FAILED -> ORDER_CANCELLED -> return stock

### Kafka Topics
- `order-events`: produced by order service
- `inventory-events`: produced by inventory service
- `payment-events`: produced by payment service

### Docker Compose
- `docker compose up --build -d` starts 3 DBs + 3 services + Redis + Kafka + Nginx
- Config per service: `config/order-service.yaml`, `config/inventory-service.yaml`, `config/payment-service.yaml`

## Code Style

- Typed sentinel errors (e.g. `ErrUserExists`, `ErrStockInsufficient`)
- Error matching with `errors.Is()`
- Table-driven tests preferred
