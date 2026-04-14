# HW5: Microservice Split & Distributed Transaction Design

## Overview

Split the flash-sale monolith into 3 independent microservices (Order, Inventory, Payment) with separate databases. Implement event-driven saga with transactional outbox for distributed transaction consistency.

**Assignment requirements covered**:
1. Order service and inventory service as independent microservices with separate databases
2. Redis-based stock pre-deduction for seckill (防超卖/限购) — already exists, preserved
3. Message-based eventual consistency for data integrity
4. Order creation + inventory deduction consistency
5. Order payment + order status update consistency

---

## 1. Service Architecture

### 1.1 Three Microservices (Logical Split)

All services live in the same Go module (`flash-sale/`) but compile to separate binaries:

```
flash-sale/
├── cmd/
│   ├── order-service/main.go       # Order + Seckill + Product (read context)
│   ├── inventory-service/main.go   # Inventory management
│   └── payment-service/main.go     # Simulated payment gateway
├── internal/
│   ├── order/          # Order domain (refactored from monolith)
│   ├── inventory/      # Inventory domain (refactored from monolith)
│   ├── payment/        # NEW: Simulated payment
│   ├── product/        # Stays with order service (read-only context)
│   ├── user/           # Stays with order service (auth)
│   └── middleware/     # JWT auth (shared)
├── pkg/
│   ├── kafka/          # Shared Kafka producer/consumer (existing)
│   ├── outbox/         # NEW: Transactional outbox pattern
│   ├── event/          # NEW: Event type definitions & payloads
│   ├── database/       # Existing DB setup
│   ├── cache/          # Existing Redis wrapper
│   ├── snowflake/      # Existing distributed ID generator
│   ├── config/         # Existing config management
│   └── response/       # Existing JSON response envelope
```

### 1.2 Database Separation

Each service owns its database exclusively. No cross-service DB access.

| Service | Database | Tables |
|---------|----------|--------|
| Order Service | `order_db` (port 5441) | users, products, orders, outbox_events |
| Inventory Service | `inventory_db` (port 5442) | inventory, outbox_events |
| Payment Service | `payment_db` (port 5443) | payments, outbox_events |

**Note**: `users` and `products` tables remain in order_db because they serve as read-only context for the order domain. The existing read-write splitting (primary/replica) applies to order_db.

### 1.3 API Routing (Nginx)

Nginx routes requests to the appropriate service:

| Path Pattern | Target Service | Port |
|-------------|---------------|------|
| `/api/v1/auth/*`, `/api/v1/user/*` | Order Service | 8081 |
| `/api/v1/products/*` | Order Service | 8081 |
| `/api/v1/orders/*`, `/api/v1/seckill/*` | Order Service | 8081 |
| `/api/v1/inventory/*` | Inventory Service | 8082 |
| `/` (static files) | Nginx (direct) | 80 |

Payment service has no external API — it communicates only via Kafka.

---

## 2. Transactional Outbox Pattern

### 2.1 Outbox Table Schema

Each service has its own `outbox_events` table:

```sql
CREATE TABLE outbox_events (
    id              BIGSERIAL PRIMARY KEY,
    aggregate_type  VARCHAR(50)  NOT NULL,   -- 'order', 'inventory', 'payment'
    aggregate_id    VARCHAR(100) NOT NULL,   -- order_no or product_id
    event_type      VARCHAR(50)  NOT NULL,   -- e.g. 'ORDER_CREATED'
    payload         JSONB        NOT NULL,   -- event-specific data
    created_at      TIMESTAMP    DEFAULT NOW(),
    sent            BOOLEAN      DEFAULT FALSE,
    sent_at         TIMESTAMP
);

CREATE INDEX idx_outbox_unsent ON outbox_events (sent, created_at) WHERE sent = FALSE;
```

### 2.2 Outbox Relay

A background goroutine in each service:

1. Poll `outbox_events` where `sent = FALSE`, ordered by `created_at`, batch of 100
2. Publish each event to the appropriate Kafka topic
3. Mark `sent = TRUE` and set `sent_at`
4. Poll interval: 500ms (configurable)
5. On Kafka publish failure: skip and retry next poll cycle

**Implementation**: `pkg/outbox/relay.go`

```go
type OutboxRelay struct {
    db       *gorm.DB
    producer *kafka.Producer
    topic    string
    interval time.Duration
}

func (r *OutboxRelay) Start(ctx context.Context)  // blocking, polls in loop
func (r *OutboxRelay) Stop()                       // graceful shutdown
```

### 2.3 How It Guarantees Consistency

Business operation + outbox write happen in the **same DB transaction**:

```go
err := db.Transaction(func(tx *gorm.DB) error {
    // 1. Business operation
    tx.Create(&order)
    // 2. Outbox event (same transaction)
    tx.Create(&OutboxEvent{
        AggregateType: "order",
        AggregateID:   order.OrderNo,
        EventType:     "ORDER_CREATED",
        Payload:       marshalJSON(orderCreatedPayload),
    })
    return nil
})
// Both succeed or both fail — atomic.
```

The relay picks up unsent events and publishes to Kafka. If the relay crashes, unsent events persist in the DB and will be relayed on restart. This guarantees **at-least-once delivery**.

Consumer-side idempotency handles duplicates (via event ID deduplication or business-level idempotency checks).

---

## 3. Event Definitions

### 3.1 Kafka Topics

| Topic | Producer | Consumers |
|-------|----------|-----------|
| `order-events` | Order Service | Inventory Service, Payment Service |
| `inventory-events` | Inventory Service | Order Service |
| `payment-events` | Payment Service | Order Service |

### 3.2 Event Types & Payloads

**ORDER_CREATED** (order-events):
```json
{
    "event_id": "unique-uuid",
    "event_type": "ORDER_CREATED",
    "order_no": "1234567890",
    "user_id": 1,
    "product_id": 5,
    "quantity": 1,
    "total_price": 99.99,
    "created_at": "2026-04-15T10:00:00Z"
}
```

**STOCK_DEDUCTED** (inventory-events):
```json
{
    "event_id": "unique-uuid",
    "event_type": "STOCK_DEDUCTED",
    "order_no": "1234567890",
    "product_id": 5,
    "quantity": 1
}
```

**STOCK_DEDUCT_FAILED** (inventory-events):
```json
{
    "event_id": "unique-uuid",
    "event_type": "STOCK_DEDUCT_FAILED",
    "order_no": "1234567890",
    "product_id": 5,
    "reason": "insufficient stock"
}
```

**PAYMENT_REQUESTED** (order-events):
```json
{
    "event_id": "unique-uuid",
    "event_type": "PAYMENT_REQUESTED",
    "order_no": "1234567890",
    "user_id": 1,
    "amount": 99.99
}
```

**PAYMENT_SUCCESS** / **PAYMENT_FAILED** (payment-events):
```json
{
    "event_id": "unique-uuid",
    "event_type": "PAYMENT_SUCCESS",
    "order_no": "1234567890",
    "payment_id": "pay-uuid-123",
    "amount": 99.99
}
```

**ORDER_COMPLETED** (order-events):
```json
{
    "event_id": "unique-uuid",
    "event_type": "ORDER_COMPLETED",
    "order_no": "1234567890",
    "product_id": 5,
    "quantity": 1
}
```

**ORDER_CANCELLED** (order-events):
```json
{
    "event_id": "unique-uuid",
    "event_type": "ORDER_CANCELLED",
    "order_no": "1234567890",
    "product_id": 5,
    "quantity": 1,
    "reason": "payment_failed"
}
```

---

## 4. Saga Flow

### 4.1 Order Status State Machine

```
PENDING(0) ──→ AWAITING_PAYMENT(1) ──→ PAID(2)
    │                   │
    ▼                   ▼
CANCELLED(3)      CANCELLED(3)
```

Transitions:
- PENDING -> AWAITING_PAYMENT: stock deducted successfully
- PENDING -> CANCELLED: stock deduction failed
- AWAITING_PAYMENT -> PAID: payment succeeded
- AWAITING_PAYMENT -> CANCELLED: payment failed

### 4.2 Happy Path (Seckill)

```
Step 1: User → POST /api/v1/seckill
  Order Service:
    - Redis SETNX idempotency check (existing)
    - Redis DECRBY stock pre-deduction (existing, fast path)
    - BEGIN TX:
      - INSERT order (status=PENDING)
      - INSERT outbox: ORDER_CREATED
    - COMMIT
    - Redis SET seckill:result:{order_no} = "PENDING"
    - Return {order_no, "processing"}

Step 2: Outbox Relay → Kafka(order-events) → Inventory Service
  Inventory Service:
    - Consume ORDER_CREATED
    - BEGIN TX:
      - UPDATE inventory: deduct with optimistic lock
      - INSERT outbox: STOCK_DEDUCTED
    - COMMIT

Step 3: Outbox Relay → Kafka(inventory-events) → Order Service
  Order Service:
    - Consume STOCK_DEDUCTED
    - BEGIN TX:
      - UPDATE order status: PENDING → AWAITING_PAYMENT
      - INSERT outbox: PAYMENT_REQUESTED
    - COMMIT

Step 4: Outbox Relay → Kafka(order-events) → Payment Service
  Payment Service:
    - Consume PAYMENT_REQUESTED
    - Simulate payment (100-500ms delay, succeed)
    - BEGIN TX:
      - INSERT payment record
      - INSERT outbox: PAYMENT_SUCCESS
    - COMMIT

Step 5: Outbox Relay → Kafka(payment-events) → Order Service
  Order Service:
    - Consume PAYMENT_SUCCESS
    - BEGIN TX:
      - UPDATE order status: AWAITING_PAYMENT → PAID
      - INSERT outbox: ORDER_COMPLETED
    - COMMIT
    - Redis SET seckill:result:{order_no} = "SUCCESS"

Step 6: Outbox Relay → Kafka(order-events) → Inventory Service
  Inventory Service:
    - Consume ORDER_COMPLETED
    - Confirm deduction: move stock from "locked" to "sold"
```

### 4.3 Compensation Path: Stock Deduction Failed

```
Inventory Service publishes STOCK_DEDUCT_FAILED:

Order Service:
  - Consume STOCK_DEDUCT_FAILED
  - BEGIN TX:
    - UPDATE order status: PENDING → CANCELLED
    - INSERT outbox: ORDER_CANCELLED (for auditing)
  - COMMIT
  - Redis: rollback stock (INCRBY)
  - Redis: delete idempotency key (allow retry)
  - Redis SET seckill:result:{order_no} = "FAILED"
```

### 4.4 Compensation Path: Payment Failed

```
Payment Service publishes PAYMENT_FAILED:

Order Service:
  - Consume PAYMENT_FAILED
  - BEGIN TX:
    - UPDATE order status: AWAITING_PAYMENT → CANCELLED
    - INSERT outbox: ORDER_CANCELLED
  - COMMIT
  - Redis SET seckill:result:{order_no} = "FAILED"

Inventory Service:
  - Consume ORDER_CANCELLED
  - Return stock: locked → available
  - Redis: rollback stock (INCRBY)
  - Redis: delete idempotency key
```

### 4.5 Idempotency in Consumers

Each consumer checks `event_id` before processing:
- Maintain a `processed_events` set in Redis (TTL 24h) or check business state
- If event already processed, skip (ack the Kafka message but do nothing)
- This handles the at-least-once delivery guarantee of the outbox relay

---

## 5. Payment Service (Simulated)

### 5.1 Behavior

- Listens on `order-events` topic for `PAYMENT_REQUESTED`
- Simulates processing: `time.Sleep(100-500ms random)`
- Configurable success rate via environment variable `PAYMENT_SUCCESS_RATE` (default: 1.0 = always succeed)
- Publishes `PAYMENT_SUCCESS` or `PAYMENT_FAILED` to `payment-events`

### 5.2 Payment Model

```go
type Payment struct {
    ID        uint      `gorm:"primaryKey"`
    PaymentID string    `gorm:"uniqueIndex"` // UUID
    OrderNo   string    `gorm:"index"`
    UserID    uint
    Amount    float64
    Status    int16     // 0=pending, 1=success, 2=failed
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## 6. Docker Topology

### 6.1 Services

```yaml
services:
  # Databases
  order-db:         # PostgreSQL, port 5441
  order-db-replica: # PostgreSQL replica, port 5444 (read-write splitting)
  inventory-db:     # PostgreSQL, port 5442
  payment-db:       # PostgreSQL, port 5443

  # Infrastructure
  redis:            # port 6379 (shared)
  zookeeper:        # port 2181
  kafka:            # port 9092

  # Microservices
  order-service:      # port 8081
  inventory-service:  # port 8082
  payment-service:    # port 8083

  # Gateway
  nginx:              # port 80
```

### 6.2 Kafka Consumer Groups

Each service uses a distinct consumer group ID to ensure independent consumption:

| Service | Consumer Group ID | Topics Consumed |
|---------|------------------|-----------------|
| Order Service | `order-service-group` | `inventory-events`, `payment-events` |
| Inventory Service | `inventory-service-group` | `order-events` |
| Payment Service | `payment-service-group` | `order-events` |

### 6.3 Database Replication

The `order-db-replica` uses PostgreSQL streaming replication from `order-db` (primary). The replica is read-only and used by the existing GORM DBResolver for read-write splitting. Inventory and payment databases are single-instance (no replication needed for this homework scope).

### 6.4 Service Configuration

Each service has its own config section or separate YAML:

**Order Service** (`config/order-service.yaml`):
- Connects to: order-db, order-db-replica, redis, kafka
- Kafka topics: produces to `order-events`, consumes `inventory-events` and `payment-events`
- Outbox relay: enabled, interval 500ms

**Inventory Service** (`config/inventory-service.yaml`):
- Connects to: inventory-db, kafka
- Kafka topics: produces to `inventory-events`, consumes `order-events`
- Outbox relay: enabled, interval 500ms

**Payment Service** (`config/payment-service.yaml`):
- Connects to: payment-db, kafka
- Kafka topics: produces to `payment-events`, consumes `order-events`
- Outbox relay: enabled, interval 500ms
- Payment success rate: configurable (default 1.0)

---

## 7. What Changes From Current Code

### 7.1 Preserved (No Changes)

- Redis seckill pre-deduction logic
- Redis idempotency check (SETNX)
- Snowflake ID generation
- Product caching (cache-aside with 3 protections)
- JWT authentication
- Frontend SPA
- JMeter test plan structure

### 7.2 Refactored

- `cmd/server/main.go` → split into 3 `cmd/*/main.go` entry points
- `internal/order/seckill_service.go` → uses outbox instead of direct Kafka publish
- `internal/order/seckill_consumer.go` → becomes event handler consuming from multiple topics
- `internal/inventory/service.go` → exposed as HTTP service + Kafka event handler
- `docker-compose.yaml` → expanded with 3 databases and 3 services
- `nginx/nginx.conf` → updated routing rules

### 7.3 New Components

- `pkg/outbox/` — outbox model, repository, relay goroutine
- `pkg/event/` — event type constants, payload structs, serialization
- `internal/payment/` — payment model, service, Kafka handler
- `internal/order/event_handler.go` — handles incoming inventory/payment events
- `internal/inventory/event_handler.go` — handles incoming order events
- Config files per service

---

## 8. Testing Strategy

- **Unit tests**: Outbox relay logic, event serialization, saga state transitions
- **Integration tests**: Full saga flow with Docker Compose (use `docker-compose.dev.yaml`)
- **JMeter**: Updated test plan with seckill load test, verify order status transitions through PENDING → PAID
- **Failure testing**: Set `PAYMENT_SUCCESS_RATE=0.5` to trigger compensation paths
