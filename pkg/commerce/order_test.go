package commerce

import (
	"testing"
)

func TestCreateOrder_Success(t *testing.T) {
	mgr := NewInMemoryOrderManager()

	items := []OrderItem{
		{
			SKU:            "ITEM001",
			Description:    "Widget",
			Quantity:       2,
			UnitPricePaise: 50_000, // ₹500
		},
		{
			SKU:            "ITEM002",
			Description:    "Gadget",
			Quantity:       1,
			UnitPricePaise: 100_000, // ₹1000
		},
	}

	order, err := mgr.CreateOrder("merchant-123", "customer-456", items)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	if order.ID == "" {
		t.Error("order.ID should not be empty")
	}
	if order.MerchantID != "merchant-123" {
		t.Errorf("want MerchantID=merchant-123, got %s", order.MerchantID)
	}
	if order.CustomerID != "customer-456" {
		t.Errorf("want CustomerID=customer-456, got %s", order.CustomerID)
	}
	// Total = (2 * 50_000) + (1 * 100_000) = 100_000 + 100_000 = 200_000
	if order.TotalPaise != 200_000 {
		t.Errorf("want TotalPaise=200000, got %d", order.TotalPaise)
	}
	if order.Status != StatusPending {
		t.Errorf("want Status=pending, got %s", order.Status)
	}
	if order.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestCreateOrder_ValidationErrors(t *testing.T) {
	mgr := NewInMemoryOrderManager()

	tests := []struct {
		name       string
		merchantID string
		customerID string
		items      []OrderItem
		wantErr    bool
	}{
		{
			name:       "missing merchant_id",
			merchantID: "",
			customerID: "customer-1",
			items:      []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}},
			wantErr:    true,
		},
		{
			name:       "missing customer_id",
			merchantID: "merchant-1",
			customerID: "",
			items:      []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}},
			wantErr:    true,
		},
		{
			name:       "empty items",
			merchantID: "merchant-1",
			customerID: "customer-1",
			items:      []OrderItem{},
			wantErr:    true,
		},
		{
			name:       "missing SKU",
			merchantID: "merchant-1",
			customerID: "customer-1",
			items:      []OrderItem{{SKU: "", Quantity: 1, UnitPricePaise: 100}},
			wantErr:    true,
		},
		{
			name:       "zero quantity",
			merchantID: "merchant-1",
			customerID: "customer-1",
			items:      []OrderItem{{SKU: "A", Quantity: 0, UnitPricePaise: 100}},
			wantErr:    true,
		},
		{
			name:       "negative quantity",
			merchantID: "merchant-1",
			customerID: "customer-1",
			items:      []OrderItem{{SKU: "A", Quantity: -1, UnitPricePaise: 100}},
			wantErr:    true,
		},
		{
			name:       "negative price",
			merchantID: "merchant-1",
			customerID: "customer-1",
			items:      []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: -100}},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mgr.CreateOrder(tt.merchantID, tt.customerID, tt.items)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateOrder error mismatch: want err=%v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGetOrder(t *testing.T) {
	mgr := NewInMemoryOrderManager()

	// Create an order
	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}}
	order, _ := mgr.CreateOrder("merchant-1", "customer-1", items)
	orderID := order.ID

	// Retrieve it
	retrieved, err := mgr.GetOrder(orderID)
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}
	if retrieved.ID != orderID {
		t.Errorf("want ID=%s, got %s", orderID, retrieved.ID)
	}

	// Try to retrieve non-existent order
	_, err = mgr.GetOrder("non-existent")
	if err == nil {
		t.Error("GetOrder should fail for non-existent order")
	}
}

func TestUpdateStatus_ValidTransitions(t *testing.T) {
	mgr := NewInMemoryOrderManager()

	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}}
	order, _ := mgr.CreateOrder("merchant-1", "customer-1", items)

	// pending -> paid
	updated, err := mgr.UpdateStatus(order.ID, StatusPaid)
	if err != nil {
		t.Fatalf("UpdateStatus(pending->paid) failed: %v", err)
	}
	if updated.Status != StatusPaid {
		t.Errorf("want Status=paid, got %s", updated.Status)
	}
	if updated.PaidAt.IsZero() {
		t.Error("PaidAt should be set when status changes to paid")
	}

	// paid -> fulfilled
	updated, err = mgr.UpdateStatus(order.ID, StatusFulfilled)
	if err != nil {
		t.Fatalf("UpdateStatus(paid->fulfilled) failed: %v", err)
	}
	if updated.Status != StatusFulfilled {
		t.Errorf("want Status=fulfilled, got %s", updated.Status)
	}
}

func TestUpdateStatus_InvalidTransitions(t *testing.T) {
	mgr := NewInMemoryOrderManager()

	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}}
	order, _ := mgr.CreateOrder("merchant-1", "customer-1", items)

	// pending -> fulfilled (invalid, must go through paid first)
	_, err := mgr.UpdateStatus(order.ID, StatusFulfilled)
	if err == nil {
		t.Error("UpdateStatus(pending->fulfilled) should fail")
	}

	// Mark as cancelled (valid from pending)
	mgr.UpdateStatus(order.ID, StatusCancelled)

	// cancelled -> anything (invalid, terminal state)
	_, err = mgr.UpdateStatus(order.ID, StatusPaid)
	if err == nil {
		t.Error("UpdateStatus(cancelled->paid) should fail")
	}
}

func TestListOrders(t *testing.T) {
	mgr := NewInMemoryOrderManager()

	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}}

	// Create 3 orders for merchant-1, 2 for merchant-2
	for i := 0; i < 3; i++ {
		mgr.CreateOrder("merchant-1", "customer-1", items)
	}
	for i := 0; i < 2; i++ {
		mgr.CreateOrder("merchant-2", "customer-2", items)
	}

	// List for merchant-1
	orders, err := mgr.ListOrders("merchant-1")
	if err != nil {
		t.Fatalf("ListOrders failed: %v", err)
	}
	if len(orders) != 3 {
		t.Errorf("want 3 orders for merchant-1, got %d", len(orders))
	}

	// List for merchant-2
	orders, err = mgr.ListOrders("merchant-2")
	if err != nil {
		t.Fatalf("ListOrders failed: %v", err)
	}
	if len(orders) != 2 {
		t.Errorf("want 2 orders for merchant-2, got %d", len(orders))
	}

	// List for merchant-3 (doesn't exist)
	orders, err = mgr.ListOrders("merchant-3")
	if err != nil {
		t.Fatalf("ListOrders failed: %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("want 0 orders for merchant-3, got %d", len(orders))
	}
}

func TestConcurrentOrderCreation(t *testing.T) {
	mgr := NewInMemoryOrderManager()
	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			mgr.CreateOrder("merchant-1", "customer-1", items)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	orders, _ := mgr.ListOrders("merchant-1")
	if len(orders) != 10 {
		t.Errorf("want 10 orders after concurrent creation, got %d", len(orders))
	}
}

func TestConcurrentStatusUpdates(t *testing.T) {
	mgr := NewInMemoryOrderManager()
	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100}}
	order, _ := mgr.CreateOrder("merchant-1", "customer-1", items)

	// Try concurrent updates (only one should succeed due to state machine rules)
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			mgr.UpdateStatus(order.ID, StatusPaymentFailed)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	// Check final state
	final, _ := mgr.GetOrder(order.ID)
	if final.Status != StatusPaymentFailed {
		t.Errorf("want final Status=payment_failed, got %s", final.Status)
	}
}
