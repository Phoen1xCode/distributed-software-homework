package outbox

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// TopicProducer can send a message to a specific Kafka topic.
type TopicProducer interface {
	SendMessageToTopic(topic string, key, value []byte) error
}

// Relay polls outbox_events and publishes unsent events to Kafka.
type Relay struct {
	db       *gorm.DB
	producer TopicProducer
	interval time.Duration
}

func NewRelay(db *gorm.DB, producer TopicProducer, interval time.Duration) *Relay {
	return &Relay{db: db, producer: producer, interval: interval}
}

// Start polls for unsent events and publishes them. Blocks until ctx is cancelled.
func (r *Relay) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.publishBatch()
		}
	}
}

func (r *Relay) publishBatch() {
	var events []OutboxEvent
	if err := r.db.Where("sent = ?", false).Order("created_at ASC").Limit(100).Find(&events).Error; err != nil {
		log.Printf("[ERROR] outbox relay: query failed: %v", err)
		return
	}

	for i := range events {
		evt := &events[i]
		if err := r.producer.SendMessageToTopic(evt.Topic, []byte(evt.AggregateID), evt.Payload); err != nil {
			log.Printf("[ERROR] outbox relay: publish event %d failed: %v", evt.ID, err)
			continue // skip this event, retry next cycle
		}

		now := time.Now()
		if err := r.db.Model(evt).Updates(map[string]interface{}{
			"sent":    true,
			"sent_at": now,
		}).Error; err != nil {
			log.Printf("[ERROR] outbox relay: mark sent %d failed: %v", evt.ID, err)
		}
	}
}
