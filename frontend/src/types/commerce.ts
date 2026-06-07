// Commerce types derived from COMMERCE_API_CONTRACTS.json.
// These match the real backend request/response shapes exactly.

// OrderItem represents a line item in an order.
export interface OrderItem {
  sku: string;
  description?: string;
  quantity: number;
  unit_price_paise: number; // paise (₹0.01 units)
}

// CreateOrderRequest matches POST /v1/commerce/order request.
export interface CreateOrderRequest {
  merchant_id: string;
  customer_id: string;
  items: OrderItem[];
}

// OrderStatus enum from the contract.
export type OrderStatus = "pending" | "paid" | "fulfilled" | "cancelled" | "payment_failed";

// Order represents a complete order (from GET /v1/commerce/order/{order_id}).
export interface Order {
  order_id: string;
  merchant_id: string;
  customer_id: string;
  items: OrderItem[];
  total_paise: number;
  status: OrderStatus;
  created_at: number; // Unix timestamp
  paid_at?: number; // Unix timestamp, optional (only set when paid)
  workflow_status?: string; // "pending" | "payment_initiated" | "payment_confirmed" | "settlement_initiated" | "settlement_completed" | "fulfilled"
}

// SettlementState enum from the contract.
export type SettlementState =
  | "pending"
  | "fetched"
  | "calculated"
  | "routed"
  | "approved"
  | "executed";

// Settlement represents a batch of orders for settlement.
export interface Settlement {
  settlement_id: string;
  state: SettlementState;
  created_at: string; // RFC3339
  updated_at: string;
  orders: Order[];
  total_amount_paise: number;
  netting_applied: boolean;
  reconciliation_status: "pending" | "verified" | "mismatch";
}

// PaymentStatus enum from the contract.
export type PaymentStatus = "pending" | "confirming" | "confirmed" | "failed" | "reversed";

// Payment represents a payment transaction.
export interface Payment {
  payment_id: string;
  order_id: string;
  from_account: string;
  to_account: string;
  amount_paise: number;
  status: PaymentStatus;
  timestamp: string; // ISO 8601
  ledger_id?: string;
}

// AuditEntry represents a lineage entry from /commerce/order/{order_id}/audit.
export interface AuditEntry {
  step: string;
  timestamp: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
}
