package order

import (
	"encoding/json"
	"flash-sale/pkg/event"
	"testing"
	"time"
)

func TestParseStockDeductedEvent(t *testing.T) {
	evt := event.StockDeductedEvent{
		BaseEvent: event.BaseEvent{
			EventID:   "test-123",
			EventType: event.StockDeducted,
			Timestamp: time.Now(),
		},
		OrderNo:   "order-001",
		ProductID: 5,
		Quantity:  1,
	}
	data, _ := json.Marshal(evt)

	var parsed event.StockDeductedEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.OrderNo != "order-001" {
		t.Errorf("OrderNo = %q, want %q", parsed.OrderNo, "order-001")
	}
}

func TestParsePaymentResultEvent(t *testing.T) {
	evt := event.PaymentResultEvent{
		BaseEvent: event.BaseEvent{
			EventID:   "test-456",
			EventType: event.PaymentSuccess,
			Timestamp: time.Now(),
		},
		OrderNo:   "order-002",
		PaymentID: "pay-789",
		Amount:    99.99,
	}
	data, _ := json.Marshal(evt)

	var parsed event.PaymentResultEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.PaymentID != "pay-789" {
		t.Errorf("PaymentID = %q, want %q", parsed.PaymentID, "pay-789")
	}
}
