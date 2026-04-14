package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"flash-sale/pkg/event"
	"flash-sale/pkg/outbox"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// EventHandler processes Kafka events for the inventory service.
type EventHandler struct {
	service *InventoryService
	db      *gorm.DB
	rdb     *redis.Client
}

func NewEventHandler(service *InventoryService, db *gorm.DB, rdb *redis.Client) *EventHandler {
	return &EventHandler{service: service, db: db, rdb: rdb}
}

// HandleOrderEvent processes events from the order-events topic.
func (h *EventHandler) HandleOrderEvent(key, value []byte) error {
	var base event.BaseEvent
	if err := json.Unmarshal(value, &base); err != nil {
		log.Printf("[ERROR] inventory: unmarshal base event: %v", err)
		return nil // don't retry malformed messages
	}

	switch base.EventType {
	case event.OrderCreated:
		return h.handleOrderCreated(value)
	case event.OrderCompleted:
		return h.handleOrderCompleted(value)
	case event.OrderCancelled:
		return h.handleOrderCancelled(value)
	default:
		log.Printf("[WARN] inventory: unknown event type: %s", base.EventType)
		return nil
	}
}

func (h *EventHandler) handleOrderCreated(data []byte) error {
	var evt event.OrderCreatedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Printf("[ERROR] inventory: unmarshal ORDER_CREATED: %v", err)
		return nil
	}

	log.Printf("[INFO] inventory: processing ORDER_CREATED for order %s, product %d, qty %d",
		evt.OrderNo, evt.ProductID, evt.Quantity)

	err := h.db.Transaction(func(tx *gorm.DB) error {
		txService := h.service.WithDB(tx)
		if err := txService.Deduct(evt.ProductID, evt.Quantity); err != nil {
			// Deduction failed — publish STOCK_DEDUCT_FAILED
			return outbox.WriteEvent(tx, "inventory", evt.OrderNo, event.StockDeductFailed, event.TopicInventoryEvents,
				event.StockDeductFailedEvent{
					BaseEvent: event.BaseEvent{
						EventID:   uuid.New().String(),
						EventType: event.StockDeductFailed,
						Timestamp: time.Now(),
					},
					OrderNo:   evt.OrderNo,
					ProductID: evt.ProductID,
					Reason:    err.Error(),
				})
		}
		// Deduction succeeded — publish STOCK_DEDUCTED
		return outbox.WriteEvent(tx, "inventory", evt.OrderNo, event.StockDeducted, event.TopicInventoryEvents,
			event.StockDeductedEvent{
				BaseEvent: event.BaseEvent{
					EventID:   uuid.New().String(),
					EventType: event.StockDeducted,
					Timestamp: time.Now(),
				},
				OrderNo:   evt.OrderNo,
				ProductID: evt.ProductID,
				Quantity:  evt.Quantity,
			})
	})
	if err != nil {
		log.Printf("[ERROR] inventory: ORDER_CREATED transaction failed: %v", err)
	}
	return nil
}

func (h *EventHandler) handleOrderCompleted(data []byte) error {
	var evt event.OrderCompletedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Printf("[ERROR] inventory: unmarshal ORDER_COMPLETED: %v", err)
		return nil
	}

	log.Printf("[INFO] inventory: confirming deduction for order %s", evt.OrderNo)
	if err := h.service.ConfirmDeduction(evt.ProductID, evt.Quantity); err != nil {
		log.Printf("[ERROR] inventory: confirm deduction failed: %v", err)
	}
	return nil
}

func (h *EventHandler) handleOrderCancelled(data []byte) error {
	var evt event.OrderCancelledEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Printf("[ERROR] inventory: unmarshal ORDER_CANCELLED: %v", err)
		return nil
	}

	log.Printf("[INFO] inventory: returning stock for cancelled order %s", evt.OrderNo)
	if err := h.service.Return(evt.ProductID, evt.Quantity); err != nil {
		log.Printf("[ERROR] inventory: return stock failed: %v", err)
	}

	// Rollback Redis stock + delete idempotency key
	RollbackRedis(h.rdb, evt.ProductID, evt.Quantity)
	idempotencyKey := fmt.Sprintf("seckill:lock:%d:%d", evt.UserID, evt.ProductID)
	h.rdb.Del(context.Background(), idempotencyKey)
	return nil
}
