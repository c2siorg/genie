import { describe, it, expect, beforeEach, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useOrder, useCreateOrder, useOrderAudit, useSettlement } from "./useCommerce";
import { setCsrfToken } from "../lib/api";

// Mock apiFetch
vi.mock("../lib/api", async () => {
  const actual = await vi.importActual("../lib/api");
  return {
    ...actual,
    apiFetch: vi.fn(),
  };
});

import { apiFetch } from "../lib/api";

describe("useCommerce hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setCsrfToken(null);
  });

  describe("useOrder", () => {
    it("fetches an order by ID", async () => {
      const mockOrder = {
        order_id: "ord-001",
        merchant_id: "m1",
        customer_id: "c1",
        items: [],
        total_paise: 100000,
        status: "pending",
        created_at: 1000,
      };
      (apiFetch as any).mockResolvedValue(mockOrder);

      const { result } = renderHook(() => useOrder("ord-001"));
      expect(result.current.order).toBeNull();

      await result.current.fetch();

      await waitFor(() => {
        expect(result.current.order).toEqual(mockOrder);
      });
    });

    it("does not fetch when orderId is null", async () => {
      const { result } = renderHook(() => useOrder(null));
      await result.current.fetch();
      expect(apiFetch).not.toHaveBeenCalled();
    });

    it("sets error on fetch failure", async () => {
      (apiFetch as any).mockRejectedValue(new Error("Network error"));

      const { result } = renderHook(() => useOrder("ord-001"));
      await result.current.fetch();

      await waitFor(() => {
        expect(result.current.error).toBeTruthy();
      });
    });
  });

  describe("useCreateOrder", () => {
    it("creates an order and returns it", async () => {
      const mockOrder = {
        order_id: "ord-new",
        merchant_id: "m1",
        customer_id: "c1",
        items: [{ sku: "SKU", quantity: 1, unit_price_paise: 50000 }],
        total_paise: 50000,
        status: "pending",
        created_at: 2000,
      };
      (apiFetch as any).mockResolvedValue(mockOrder);

      const { result } = renderHook(() => useCreateOrder());
      const created = await result.current.create({
        merchant_id: "m1",
        customer_id: "c1",
        items: [{ sku: "SKU", quantity: 1, unit_price_paise: 50000 }],
      });

      expect(created).toEqual(mockOrder);
      expect(apiFetch).toHaveBeenCalledWith("/commerce/order", {
        method: "POST",
        json: expect.objectContaining({
          merchant_id: "m1",
          customer_id: "c1",
        }),
      });
    });

    it("returns null on error", async () => {
      (apiFetch as any).mockRejectedValue(new Error("Validation failed"));

      const { result } = renderHook(() => useCreateOrder());
      const created = await result.current.create({
        merchant_id: "m1",
        customer_id: "c1",
        items: [],
      });

      // If error occurred, create returns null
      expect(created).toBeNull();
    });
  });

  describe("useOrderAudit", () => {
    it("fetches audit entries for an order", async () => {
      const mockAuditResponse = {
        entries: [
          { step: "created", timestamp: "2026-01-01T00:00:00Z" },
          { step: "payment_initiated", timestamp: "2026-01-01T00:01:00Z" },
        ],
      };
      (apiFetch as any).mockResolvedValue(mockAuditResponse);

      const { result } = renderHook(() => useOrderAudit("ord-001"));
      expect(result.current.entries).toHaveLength(0);

      await result.current.fetch();

      await waitFor(() => {
        expect(result.current.entries).toHaveLength(2);
        expect(result.current.entries[0].step).toBe("created");
      });
    });

    it("handles missing entries response", async () => {
      (apiFetch as any).mockResolvedValue({});

      const { result } = renderHook(() => useOrderAudit("ord-001"));
      await result.current.fetch();

      await waitFor(() => {
        expect(result.current.entries).toHaveLength(0);
      });
    });
  });

  describe("useSettlement", () => {
    it("fetches a settlement by ID", async () => {
      const mockSettlement = {
        settlement_id: "set-001",
        state: "pending",
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:01:00Z",
        orders: [],
        total_amount_paise: 100000,
        netting_applied: false,
        reconciliation_status: "pending",
      };
      (apiFetch as any).mockResolvedValue(mockSettlement);

      const { result } = renderHook(() => useSettlement("set-001"));
      expect(result.current.settlement).toBeNull();

      await result.current.fetch();

      await waitFor(() => {
        expect(result.current.settlement).toEqual(mockSettlement);
      });
    });

    it("does not fetch when settlementId is null", async () => {
      const { result } = renderHook(() => useSettlement(null));
      await result.current.fetch();
      expect(apiFetch).not.toHaveBeenCalled();
    });
  });
});
