package outbox

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockProducer struct {
	mu       sync.Mutex
	messages []struct{ topic string; key, value []byte }
}

func (m *mockProducer) SendMessageToTopic(topic string, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, struct{ topic string; key, value []byte }{topic, key, value})
	return nil
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&OutboxEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRelay_PublishesUnsentEvents(t *testing.T) {
	db := setupTestDB(t)
	mock := &mockProducer{}

	// Insert an unsent event
	payload, _ := json.Marshal(map[string]string{"order_no": "123"})
	db.Create(&OutboxEvent{
		AggregateType: "order",
		AggregateID:   "123",
		EventType:     "ORDER_CREATED",
		Topic:         "order-events",
		Payload:       payload,
		Sent:          false,
	})

	relay := NewRelay(db, mock, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go relay.Start(ctx)
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond) // let final tick complete

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	if mock.messages[0].topic != "order-events" {
		t.Errorf("topic = %q, want %q", mock.messages[0].topic, "order-events")
	}

	// Verify event marked as sent
	var evt OutboxEvent
	db.First(&evt)
	if !evt.Sent {
		t.Error("expected event to be marked as sent")
	}
	if evt.SentAt == nil {
		t.Error("expected SentAt to be set")
	}
}

func TestRelay_SkipsAlreadySentEvents(t *testing.T) {
	db := setupTestDB(t)
	mock := &mockProducer{}

	now := time.Now()
	payload, _ := json.Marshal(map[string]string{"order_no": "456"})
	db.Create(&OutboxEvent{
		AggregateType: "order",
		AggregateID:   "456",
		EventType:     "ORDER_CREATED",
		Topic:         "order-events",
		Payload:       payload,
		Sent:          true,
		SentAt:        &now,
	})

	relay := NewRelay(db, mock, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go relay.Start(ctx)
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(mock.messages))
	}
}
