package event

import "time"

// Event type constants
const (
	OrderCreated      = "ORDER_CREATED"
	OrderCompleted    = "ORDER_COMPLETED"
	OrderCancelled    = "ORDER_CANCELLED"
	PaymentRequested  = "PAYMENT_REQUESTED"
	StockDeducted     = "STOCK_DEDUCTED"
	StockDeductFailed = "STOCK_DEDUCT_FAILED"
	PaymentSuccess    = "PAYMENT_SUCCESS"
	PaymentFailed     = "PAYMENT_FAILED"
)

// Kafka topic constants
const (
	TopicOrderEvents     = "order-events"
	TopicInventoryEvents = "inventory-events"
	TopicPaymentEvents   = "payment-events"
)

// BaseEvent is embedded in all event payloads.
type BaseEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
}

type OrderCreatedEvent struct {
	BaseEvent
	OrderNo    string  `json:"order_no"`
	UserID     uint    `json:"user_id"`
	ProductID  uint    `json:"product_id"`
	Quantity   int     `json:"quantity"`
	TotalPrice float64 `json:"total_price"`
}

type OrderCompletedEvent struct {
	BaseEvent
	OrderNo   string `json:"order_no"`
	ProductID uint   `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type OrderCancelledEvent struct {
	BaseEvent
	OrderNo   string `json:"order_no"`
	UserID    uint   `json:"user_id"`
	ProductID uint   `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Reason    string `json:"reason"`
}

type PaymentRequestedEvent struct {
	BaseEvent
	OrderNo string  `json:"order_no"`
	UserID  uint    `json:"user_id"`
	Amount  float64 `json:"amount"`
}

type StockDeductedEvent struct {
	BaseEvent
	OrderNo   string `json:"order_no"`
	ProductID uint   `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type StockDeductFailedEvent struct {
	BaseEvent
	OrderNo   string `json:"order_no"`
	ProductID uint   `json:"product_id"`
	Reason    string `json:"reason"`
}

type PaymentResultEvent struct {
	BaseEvent
	OrderNo   string  `json:"order_no"`
	PaymentID string  `json:"payment_id"`
	Amount    float64 `json:"amount"`
}
