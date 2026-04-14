package event

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOrderCreatedEvent_MarshalUnmarshal(t *testing.T) {
	original := OrderCreatedEvent{
		BaseEvent: BaseEvent{
			EventID:   "test-uuid-123",
			EventType: OrderCreated,
			Timestamp: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		},
		OrderNo:    "1234567890",
		UserID:     1,
		ProductID:  5,
		Quantity:   1,
		TotalPrice: 99.99,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded OrderCreatedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.EventID != original.EventID {
		t.Errorf("EventID = %q, want %q", decoded.EventID, original.EventID)
	}
	if decoded.EventType != OrderCreated {
		t.Errorf("EventType = %q, want %q", decoded.EventType, OrderCreated)
	}
	if decoded.OrderNo != original.OrderNo {
		t.Errorf("OrderNo = %q, want %q", decoded.OrderNo, original.OrderNo)
	}
	if decoded.TotalPrice != original.TotalPrice {
		t.Errorf("TotalPrice = %v, want %v", decoded.TotalPrice, original.TotalPrice)
	}
}

func TestStockDeductedEvent_MarshalUnmarshal(t *testing.T) {
	original := StockDeductedEvent{
		BaseEvent: BaseEvent{
			EventID:   "test-uuid-456",
			EventType: StockDeducted,
			Timestamp: time.Now(),
		},
		OrderNo:   "1234567890",
		ProductID: 5,
		Quantity:  1,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded StockDeductedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.OrderNo != original.OrderNo {
		t.Errorf("OrderNo = %q, want %q", decoded.OrderNo, original.OrderNo)
	}
}

func TestPaymentSuccessEvent_MarshalUnmarshal(t *testing.T) {
	original := PaymentResultEvent{
		BaseEvent: BaseEvent{
			EventID:   "test-uuid-789",
			EventType: PaymentSuccess,
			Timestamp: time.Now(),
		},
		OrderNo:   "1234567890",
		PaymentID: "pay-uuid-123",
		Amount:    99.99,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PaymentResultEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.PaymentID != original.PaymentID {
		t.Errorf("PaymentID = %q, want %q", decoded.PaymentID, original.PaymentID)
	}
	if decoded.EventType != PaymentSuccess {
		t.Errorf("EventType = %q, want %q", decoded.EventType, PaymentSuccess)
	}
}
