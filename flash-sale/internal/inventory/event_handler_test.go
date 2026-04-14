package inventory

import (
	"encoding/json"
	"flash-sale/pkg/event"
	"testing"
	"time"
)

func TestParseOrderCreatedEvent(t *testing.T) {
	evt := event.OrderCreatedEvent{
		BaseEvent: event.BaseEvent{
			EventID:   "test-123",
			EventType: event.OrderCreated,
			Timestamp: time.Now(),
		},
		OrderNo:   "order-001",
		ProductID: 5,
		Quantity:  2,
	}
	data, _ := json.Marshal(evt)

	var parsed event.OrderCreatedEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.ProductID != 5 {
		t.Errorf("ProductID = %d, want 5", parsed.ProductID)
	}
	if parsed.Quantity != 2 {
		t.Errorf("Quantity = %d, want 2", parsed.Quantity)
	}
}
