package outbox

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type OutboxEvent struct {
	ID            uint            `gorm:"primaryKey" json:"id"`
	AggregateType string          `gorm:"size:50;not null" json:"aggregate_type"`
	AggregateID   string          `gorm:"size:100;not null" json:"aggregate_id"`
	EventType     string          `gorm:"size:50;not null" json:"event_type"`
	Topic         string          `gorm:"size:100;not null" json:"topic"`
	Payload       json.RawMessage `gorm:"type:jsonb;not null" json:"payload"`
	Sent          bool            `gorm:"default:false;index:idx_outbox_unsent" json:"sent"`
	CreatedAt     time.Time       `gorm:"autoCreateTime;index:idx_outbox_unsent" json:"created_at"`
	SentAt        *time.Time      `json:"sent_at"`
}

func (OutboxEvent) TableName() string {
	return "outbox_events"
}

// WriteEvent inserts an outbox event within the given transaction.
func WriteEvent(tx *gorm.DB, aggregateType, aggregateID, eventType, topic string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return tx.Create(&OutboxEvent{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Topic:         topic,
		Payload:       data,
	}).Error
}
