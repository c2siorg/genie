import { describe, it, expect } from "vitest";
import type { Order, OrderItem, CreateOrderRequest } from "./commerce";

describe("commerce types", () => {
  it("allows creating a valid OrderItem", () => {
    const item: OrderItem = {
      sku: "SKU-001",
      description: "Test item",
      quantity: 5,
      unit_price_paise: 10000, // ₹100
    };
    expect(item.quantity).toBe(5);
    expect(item.unit_price_paise).toBe(10000);
  });

  it("allows creating a CreateOrderRequest", () => {
    const req: CreateOrderRequest = {
      merchant_id: "merchant-123",
      customer_id: "customer-456",
      items: [
        { sku: "SKU-A", quantity: 2, unit_price_paise: 50000 },
        { sku: "SKU-B", quantity: 1, unit_price_paise: 100000 },
      ],
    };
    expect(req.items).toHaveLength(2);
    expect(req.merchant_id).toBe("merchant-123");
  });

  it("allows creating an Order", () => {
    const order: Order = {
      order_id: "ord-789",
      merchant_id: "merchant-123",
      customer_id: "customer-456",
      items: [{ sku: "SKU-X", quantity: 1, unit_price_paise: 100000 }],
      total_paise: 100000,
      status: "pending",
      created_at: Math.floor(Date.now() / 1000),
      workflow_status: "pending",
    };
    expect(order.status).toBe("pending");
    expect(order.total_paise).toBe(100000);
  });

  it("allows optional fields in Order", () => {
    const order: Order = {
      order_id: "ord-123",
      merchant_id: "m1",
      customer_id: "c1",
      items: [],
      total_paise: 0,
      status: "paid",
      created_at: 1000000,
      paid_at: 1000100, // optional
      workflow_status: "payment_confirmed",
    };
    expect(order.paid_at).toBe(1000100);
  });

  it("validates order status enum values", () => {
    const statuses = ["pending", "paid", "fulfilled", "cancelled", "payment_failed"];
    for (const status of statuses) {
      const order: Order = {
        order_id: "id",
        merchant_id: "m",
        customer_id: "c",
        items: [],
        total_paise: 0,
        status: status as any,
        created_at: 0,
      };
      expect(order.status).toBe(status);
    }
  });
});
