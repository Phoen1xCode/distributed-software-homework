package payment

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"flash-sale/pkg/event"
	"flash-sale/pkg/outbox"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service handles payment processing. It listens for PAYMENT_REQUESTED events.
type Service struct {
	db          *gorm.DB
	successRate float64 // 0.0 to 1.0
}

func NewService(db *gorm.DB, successRate float64) *Service {
	if successRate < 0 || successRate > 1 {
		successRate = 1.0
	}
	return &Service{db: db, successRate: successRate}
}

// HandleOrderEvent processes events from the order-events topic.
func (s *Service) HandleOrderEvent(key, value []byte) error {
	var base event.BaseEvent
	if err := json.Unmarshal(value, &base); err != nil {
		log.Printf("[ERROR] payment: unmarshal base event: %v", err)
		return nil
	}

	if base.EventType != event.PaymentRequested {
		return nil // ignore non-payment events
	}

	var evt event.PaymentRequestedEvent
	if err := json.Unmarshal(value, &evt); err != nil {
		log.Printf("[ERROR] payment: unmarshal PAYMENT_REQUESTED: %v", err)
		return nil
	}

	log.Printf("[INFO] payment: processing payment for order %s, amount %.2f", evt.OrderNo, evt.Amount)

	// Simulate payment processing delay
	delay := 100 + rand.Intn(400)
	time.Sleep(time.Duration(delay) * time.Millisecond)

	paymentID := uuid.New().String()
	success := rand.Float64() < s.successRate

	var status int16
	var eventType string
	if success {
		status = 1
		eventType = event.PaymentSuccess
	} else {
		status = 2
		eventType = event.PaymentFailed
	}

	p := &Payment{
		PaymentID: paymentID,
		OrderNo:   evt.OrderNo,
		UserID:    evt.UserID,
		Amount:    evt.Amount,
		Status:    status,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		return outbox.WriteEvent(tx, "payment", evt.OrderNo, eventType, event.TopicPaymentEvents,
			event.PaymentResultEvent{
				BaseEvent: event.BaseEvent{
					EventID:   uuid.New().String(),
					EventType: eventType,
					Timestamp: time.Now(),
				},
				OrderNo:   evt.OrderNo,
				PaymentID: paymentID,
				Amount:    evt.Amount,
			})
	})
	if err != nil {
		log.Printf("[ERROR] payment: transaction failed for order %s: %v", evt.OrderNo, err)
	} else {
		log.Printf("[INFO] payment: order %s payment %s (id: %s)", evt.OrderNo, eventType, paymentID)
	}
	return nil
}
