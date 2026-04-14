package order

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

// EventHandler processes incoming saga events for the order service.
type EventHandler struct {
	repo *Repository
	db   *gorm.DB
	rdb  *redis.Client
}

func NewEventHandler(repo *Repository, db *gorm.DB, rdb *redis.Client) *EventHandler {
	return &EventHandler{repo: repo, db: db, rdb: rdb}
}

// HandleInventoryEvent processes events from the inventory-events topic.
func (h *EventHandler) HandleInventoryEvent(key, value []byte) error {
	var base event.BaseEvent
	if err := json.Unmarshal(value, &base); err != nil {
		log.Printf("[ERROR] order: unmarshal base event: %v", err)
		return nil
	}

	switch base.EventType {
	case event.StockDeducted:
		return h.handleStockDeducted(value)
	case event.StockDeductFailed:
		return h.handleStockDeductFailed(value)
	default:
		log.Printf("[WARN] order: unknown inventory event: %s", base.EventType)
		return nil
	}
}

// HandlePaymentEvent processes events from the payment-events topic.
func (h *EventHandler) HandlePaymentEvent(key, value []byte) error {
	var base event.BaseEvent
	if err := json.Unmarshal(value, &base); err != nil {
		log.Printf("[ERROR] order: unmarshal base event: %v", err)
		return nil
	}

	switch base.EventType {
	case event.PaymentSuccess:
		return h.handlePaymentSuccess(value)
	case event.PaymentFailed:
		return h.handlePaymentFailed(value)
	default:
		log.Printf("[WARN] order: unknown payment event: %s", base.EventType)
		return nil
	}
}

func (h *EventHandler) handleStockDeducted(data []byte) error {
	var evt event.StockDeductedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Printf("[ERROR] order: unmarshal STOCK_DEDUCTED: %v", err)
		return nil
	}

	log.Printf("[INFO] order: stock deducted for order %s, advancing to AWAITING_PAYMENT", evt.OrderNo)

	o, err := h.repo.GetOrderByOrderNo(evt.OrderNo)
	if err != nil {
		log.Printf("[ERROR] order: get order %s: %v", evt.OrderNo, err)
		return nil
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := h.repo.UpdateStatusByOrderNoTx(tx, evt.OrderNo, StatusAwaitingPayment); err != nil {
			return err
		}
		return outbox.WriteEvent(tx, "order", evt.OrderNo, event.PaymentRequested, event.TopicOrderEvents,
			event.PaymentRequestedEvent{
				BaseEvent: event.BaseEvent{
					EventID:   uuid.New().String(),
					EventType: event.PaymentRequested,
					Timestamp: time.Now(),
				},
				OrderNo: evt.OrderNo,
				UserID:  o.UserID,
				Amount:  o.TotalPrice,
			})
	})
	if err != nil {
		log.Printf("[ERROR] order: STOCK_DEDUCTED transaction failed: %v", err)
	}
	return nil
}

func (h *EventHandler) handleStockDeductFailed(data []byte) error {
	var evt event.StockDeductFailedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Printf("[ERROR] order: unmarshal STOCK_DEDUCT_FAILED: %v", err)
		return nil
	}

	log.Printf("[INFO] order: stock deduction failed for order %s: %s", evt.OrderNo, evt.Reason)

	o, err := h.repo.GetOrderByOrderNo(evt.OrderNo)
	if err != nil {
		log.Printf("[ERROR] order: get order %s: %v", evt.OrderNo, err)
		return nil
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := h.repo.UpdateStatusByOrderNoTx(tx, evt.OrderNo, StatusCancelled); err != nil {
			return err
		}
		return outbox.WriteEvent(tx, "order", evt.OrderNo, event.OrderCancelled, event.TopicOrderEvents,
			event.OrderCancelledEvent{
				BaseEvent: event.BaseEvent{
					EventID:   uuid.New().String(),
					EventType: event.OrderCancelled,
					Timestamp: time.Now(),
				},
				OrderNo:   evt.OrderNo,
				UserID:    o.UserID,
				ProductID: evt.ProductID,
				Quantity:  o.Quantity,
				Reason:    "stock_deduction_failed",
			})
	})
	if err != nil {
		log.Printf("[ERROR] order: STOCK_DEDUCT_FAILED transaction failed: %v", err)
		return nil
	}

	ctx := context.Background()
	resultKey := fmt.Sprintf("seckill:result:%s", evt.OrderNo)
	h.rdb.Set(ctx, resultKey, "FAILED", 0)
	return nil
}

func (h *EventHandler) handlePaymentSuccess(data []byte) error {
	var evt event.PaymentResultEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Printf("[ERROR] order: unmarshal PAYMENT_SUCCESS: %v", err)
		return nil
	}

	log.Printf("[INFO] order: payment succeeded for order %s", evt.OrderNo)

	o, err := h.repo.GetOrderByOrderNo(evt.OrderNo)
	if err != nil {
		log.Printf("[ERROR] order: get order %s: %v", evt.OrderNo, err)
		return nil
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := h.repo.UpdateStatusByOrderNoTx(tx, evt.OrderNo, StatusPaid); err != nil {
			return err
		}
		return outbox.WriteEvent(tx, "order", evt.OrderNo, event.OrderCompleted, event.TopicOrderEvents,
			event.OrderCompletedEvent{
				BaseEvent: event.BaseEvent{
					EventID:   uuid.New().String(),
					EventType: event.OrderCompleted,
					Timestamp: time.Now(),
				},
				OrderNo:   evt.OrderNo,
				ProductID: o.ProductID,
				Quantity:  o.Quantity,
			})
	})
	if err != nil {
		log.Printf("[ERROR] order: PAYMENT_SUCCESS transaction failed: %v", err)
		return nil
	}

	ctx := context.Background()
	h.rdb.Set(ctx, fmt.Sprintf("seckill:result:%s", evt.OrderNo), "SUCCESS", 0)
	return nil
}

func (h *EventHandler) handlePaymentFailed(data []byte) error {
	var evt event.PaymentResultEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Printf("[ERROR] order: unmarshal PAYMENT_FAILED: %v", err)
		return nil
	}

	log.Printf("[INFO] order: payment failed for order %s", evt.OrderNo)

	o, err := h.repo.GetOrderByOrderNo(evt.OrderNo)
	if err != nil {
		log.Printf("[ERROR] order: get order %s: %v", evt.OrderNo, err)
		return nil
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := h.repo.UpdateStatusByOrderNoTx(tx, evt.OrderNo, StatusCancelled); err != nil {
			return err
		}
		return outbox.WriteEvent(tx, "order", evt.OrderNo, event.OrderCancelled, event.TopicOrderEvents,
			event.OrderCancelledEvent{
				BaseEvent: event.BaseEvent{
					EventID:   uuid.New().String(),
					EventType: event.OrderCancelled,
					Timestamp: time.Now(),
				},
				OrderNo:   evt.OrderNo,
				UserID:    o.UserID,
				ProductID: o.ProductID,
				Quantity:  o.Quantity,
				Reason:    "payment_failed",
			})
	})
	if err != nil {
		log.Printf("[ERROR] order: PAYMENT_FAILED transaction failed: %v", err)
		return nil
	}

	ctx := context.Background()
	h.rdb.Set(ctx, fmt.Sprintf("seckill:result:%s", evt.OrderNo), "FAILED", 0)
	return nil
}
