package payment

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"sync/atomic"
	"time"

	"flash-sale/pkg/event"
	"flash-sale/pkg/outbox"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service handles payment processing. It listens for PAYMENT_REQUESTED events.
// successRate is stored as float64 bits in an atomic uint64 so that a Nacos
// config-change callback can update it from another goroutine without locks.
type Service struct {
	db              *gorm.DB
	successRateBits atomic.Uint64
}

func NewService(db *gorm.DB, successRate float64) *Service {
	s := &Service{db: db}
	s.SetSuccessRate(successRate)
	return s
}

// SetSuccessRate updates the simulated payment success probability. Out-of-range
// values are clamped to 1.0 so config typos do not break the service.
func (s *Service) SetSuccessRate(rate float64) {
	if rate < 0 || rate > 1 {
		rate = 1.0
	}
	s.successRateBits.Store(math.Float64bits(rate))
}

func (s *Service) successRate() float64 {
	return math.Float64frombits(s.successRateBits.Load())
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
	success := rand.Float64() < s.successRate()

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
