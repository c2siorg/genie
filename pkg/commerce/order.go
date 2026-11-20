package commerce

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OrderManager defines the interface for order lifecycle management.
type OrderManager interface {
	// CreateOrder validates items, computes total, and creates a new Order.
	CreateOrder(merchantID, customerID string, items []OrderItem) (*Order, error)
	// GetOrder retrieves an order by ID.
	GetOrder(orderID string) (*Order, error)
	// UpdateStatus changes the order status.
	UpdateStatus(orderID string, status OrderStatus) (*Order, error)
	// ListOrders returns all orders for a merchant.
	ListOrders(merchantID string) ([]*Order, error)
}

// InMemoryOrderManager is a thread-safe in-memory order store.
type InMemoryOrderManager struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

// NewInMemoryOrderManager creates an empty order store.
func NewInMemoryOrderManager() *InMemoryOrderManager {
	return &InMemoryOrderManager{
		orders: make(map[string]*Order),
	}
}

// CreateOrder validates items, computes total, assigns ID, and stores the order.
// Items must not be empty, quantities must be positive, and prices must be non-negative.
func (m *InMemoryOrderManager) CreateOrder(merchantID, customerID string, items []OrderItem) (*Order, error) {
	if merchantID == "" {
		return nil, fmt.Errorf("merchant_id required")
	}
	if customerID == "" {
		return nil, fmt.Errorf("customer_id required")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("items required (at least one)")
	}

	// Validate items and compute total
	var totalPaise int64
	for i, item := range items {
		if item.SKU == "" {
			return nil, fmt.Errorf("item[%d]: sku required", i)
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("item[%d]: quantity must be positive", i)
		}
		if item.UnitPricePaise < 0 {
			return nil, fmt.Errorf("item[%d]: unit_price_paise cannot be negative", i)
		}
		lineTotalPaise := int64(item.Quantity) * item.UnitPricePaise
		totalPaise += lineTotalPaise
	}

	order := &Order{
		ID:         uuid.New().String(),
		MerchantID: merchantID,
		CustomerID: customerID,
		Items:      items,
		TotalPaise: totalPaise,
		Status:     StatusPending,
		CreatedAt:  time.Now().UTC(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders[order.ID] = order
	return order, nil
}

// GetOrder retrieves an order by ID. Returns nil if not found.
func (m *InMemoryOrderManager) GetOrder(orderID string) (*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	order, ok := m.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}
	// Return a copy to prevent external mutations
	orderCopy := *order
	return &orderCopy, nil
}

// UpdateStatus changes the order status. Only valid status transitions are allowed.
func (m *InMemoryOrderManager) UpdateStatus(orderID string, newStatus OrderStatus) (*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	// Validate state transition
	if !isValidTransition(order.Status, newStatus) {
		return nil, fmt.Errorf("invalid status transition: %s -> %s", order.Status, newStatus)
	}

	order.Status = newStatus
	if newStatus == StatusPaid {
		order.PaidAt = time.Now().UTC()
	}

	// Return a copy
	orderCopy := *order
	return &orderCopy, nil
}

// ListOrders returns all orders for a given merchant.
func (m *InMemoryOrderManager) ListOrders(merchantID string) ([]*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var orders []*Order
	for _, order := range m.orders {
		if order.MerchantID == merchantID {
			orderCopy := *order
			orders = append(orders, &orderCopy)
		}
	}
	return orders, nil
}

// isValidTransition checks if a status change is valid.
func isValidTransition(from, to OrderStatus) bool {
	validTransitions := map[OrderStatus]map[OrderStatus]bool{
		StatusPending: {
			StatusPaid:          true,
			StatusPaymentFailed: true,
			StatusCancelled:     true,
		},
		StatusPaid: {
			StatusFulfilled:     true,
			StatusPaymentFailed: true,
			StatusCancelled:     true,
		},
		StatusPaymentFailed: {
			StatusPaid:      true,
			StatusCancelled: true,
		},
		StatusFulfilled: {
			StatusCancelled: true, // rare, but allow refund
		},
		StatusCancelled: {}, // terminal
	}

	transitions, ok := validTransitions[from]
	if !ok {
		return false
	}
	return transitions[to]
}
