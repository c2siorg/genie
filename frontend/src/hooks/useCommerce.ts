// useCommerce hooks for the Commerce workspace.
// All calls use the real API client (with CSRF + cookies).

import { useCallback, useState } from "react";
import { apiFetch, ApiError } from "../lib/api";
import type { Order, CreateOrderRequest, Settlement, AuditEntry } from "../types/commerce";

// useOrder fetches a single order by ID.
export function useOrder(orderId: string | null) {
  const [order, setOrder] = useState<Order | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!orderId) return;
    setLoading(true);
    try {
      const o = await apiFetch<Order>(`/commerce/order/${orderId}`);
      setOrder(o);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [orderId]);

  return { order, loading, error, fetch };
}

// useCreateOrder sends a CreateOrderRequest and returns the created order.
export function useCreateOrder() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = useCallback(async (req: CreateOrderRequest): Promise<Order | null> => {
    setLoading(true);
    try {
      const o = await apiFetch<Order>("/commerce/order", {
        method: "POST",
        json: req,
      });
      setError(null);
      return o;
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  return { create, loading, error };
}

// useSettlement fetches a settlement by ID.
export function useSettlement(settlementId: string | null) {
  const [settlement, setSettlement] = useState<Settlement | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!settlementId) return;
    setLoading(true);
    try {
      const s = await apiFetch<Settlement>(`/settlement/request/${settlementId}`);
      setSettlement(s);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [settlementId]);

  return { settlement, loading, error, fetch };
}

// useOrderAudit fetches the audit trail for an order.
export function useOrderAudit(orderId: string | null) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!orderId) return;
    setLoading(true);
    try {
      const response = await apiFetch<{
        entries: AuditEntry[];
      }>(`/commerce/order/${orderId}/audit`);
      setEntries(response.entries || []);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, [orderId]);

  return { entries, loading, error, fetch };
}

// useExecuteOrder runs the order workflow (payment → settlement).
export function useExecuteOrder() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const execute = useCallback(async (orderId: string): Promise<boolean> => {
    setLoading(true);
    try {
      await apiFetch(`/commerce/order/${orderId}/execute`, {
        method: "POST",
        json: {},
      });
      setError(null);
      return true;
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unknown error");
      return false;
    } finally {
      setLoading(false);
    }
  }, []);

  return { execute, loading, error };
}
