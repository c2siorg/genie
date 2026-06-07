// Workspace pages. Phase 2+: Commerce is real; others are stubs awaiting their phases.

import { Money } from "../components/Money";
import { StateMachine } from "../components/StateMachine";
import { useProvenance } from "../provenance/provenance";

function Stub({ title, blurb }: { title: string; blurb: string }) {
  return (
    <section>
      <h1>{title}</h1>
      <p style={{ color: "var(--color-text-muted)" }}>{blurb}</p>
      <p style={{ color: "var(--color-text-muted)" }}>
        This workspace is scaffolded in Phase 1; its features land in a later phase.
      </p>
    </section>
  );
}

export const Assistant = () => (
  <Stub title="Assistant" blurb="Conversational financial assistant (Phase 6)." />
);

import { useState, useEffect } from "react";
import type { CreateOrderRequest } from "../types/commerce";
import { useOrder, useCreateOrder, useOrderAudit } from "../hooks/useCommerce";

const SETTLEMENT_STEPS = [
  { key: "created", label: "Created", detail: "12:00" },
  { key: "payment_initiated", label: "Payment", detail: "12:01" },
  { key: "payment_confirmed", label: "Confirmed", detail: "12:01" },
  { key: "settlement", label: "Settlement", detail: "12:02" },
  { key: "fulfilled", label: "Fulfilled" },
];

export function Commerce() {
  const { open } = useProvenance();
  const [demoOrderId, setDemoOrderId] = useState<string | null>(null);
  const { order, loading: loadingOrder, error: orderError, fetch: fetchOrder } =
    useOrder(demoOrderId);
  const { create, loading: creatingOrder } = useCreateOrder();
  const { entries: auditEntries } = useOrderAudit(demoOrderId);

  const handleCreateDemoOrder = async () => {
    const req: CreateOrderRequest = {
      merchant_id: "merchant-demo",
      customer_id: "customer-demo",
      items: [
        {
          sku: "SKU-DEMO-001",
          description: "Genie Demo Item",
          quantity: 1,
          unit_price_paise: 10000000, // ₹1,00,000.00
        },
      ],
    };
    const created = await create(req);
    if (created) {
      setDemoOrderId(created.order_id);
    }
  };

  useEffect(() => {
    if (demoOrderId) {
      fetchOrder();
    }
  }, [demoOrderId, fetchOrder]);

  return (
    <section>
      <h1>Commerce</h1>
      <p style={{ color: "var(--color-text-muted)" }}>
        Orders, payments, settlement &amp; payouts — Phase 2 real API integration.
      </p>

      {!order && (
        <button
          onClick={handleCreateDemoOrder}
          disabled={creatingOrder}
          style={{
            marginTop: "var(--space-4)",
            padding: "var(--space-2) var(--space-4)",
            background: "var(--color-info)",
            color: "white",
            border: "none",
            borderRadius: "var(--radius)",
            cursor: "pointer",
          }}
        >
          {creatingOrder ? "Creating order..." : "Create demo order"}
        </button>
      )}

      {orderError && (
        <div style={{ color: "var(--color-danger)", marginTop: "var(--space-4)" }}>
          Error: {orderError}
        </div>
      )}

      {loadingOrder && (
        <p style={{ color: "var(--color-text-muted)", marginTop: "var(--space-4)" }}>
          Loading order...
        </p>
      )}

      {order && (
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: "var(--radius)",
            padding: "var(--space-6)",
            marginTop: "var(--space-4)",
          }}
        >
          {/* Order Header */}
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              marginBottom: "var(--space-4)",
            }}
          >
            <div>
              <strong>Order {order.order_id}</strong>
              <p style={{ color: "var(--color-text-muted)", fontSize: "0.9rem" }}>
                {new Date(order.created_at * 1000).toLocaleString()}
              </p>
            </div>
            <div style={{ textAlign: "right" }}>
              <Money paise={order.total_paise} intent="credit" />
              <p
                style={{
                  color: "var(--color-text-muted)",
                  fontSize: "0.9rem",
                  marginTop: "var(--space-1)",
                }}
              >
                Status: {order.status}
              </p>
            </div>
          </div>

          {/* Settlement Timeline */}
          <div style={{ margin: "var(--space-4) 0" }}>
            <p style={{ color: "var(--color-text-muted)", fontSize: "0.85rem" }}>
              Workflow Status
            </p>
            <StateMachine
              steps={SETTLEMENT_STEPS}
              current={order.workflow_status || "created"}
              aria-label={`Settlement status for ${order.order_id}`}
            />
          </div>

          {/* Items */}
          <div style={{ marginTop: "var(--space-4)" }}>
            <p style={{ color: "var(--color-text-muted)", fontSize: "0.85rem" }}>
              Items
            </p>
            {order.items.map((item, i) => (
              <div
                key={i}
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  padding: "var(--space-2) 0",
                  borderBottom:
                    i < order.items.length - 1 ? `1px solid var(--color-border)` : "none",
                }}
              >
                <span>
                  {item.quantity}x {item.sku}
                </span>
                <Money paise={item.unit_price_paise * item.quantity} />
              </div>
            ))}
          </div>

          {/* Provenance */}
          <button
            onClick={() => open(order.order_id)}
            style={{
              marginTop: "var(--space-4)",
              padding: "var(--space-2) var(--space-3)",
              background: "transparent",
              border: `1px solid var(--color-border)`,
              color: "var(--color-text)",
              borderRadius: "var(--radius)",
              cursor: "pointer",
            }}
          >
            Why? · View provenance ↗
          </button>

          {/* Audit Trail */}
          {auditEntries.length > 0 && (
            <div style={{ marginTop: "var(--space-6)" }}>
              <p style={{ color: "var(--color-text-muted)", fontSize: "0.85rem" }}>
                Audit Trail ({auditEntries.length} entries)
              </p>
              {auditEntries.map((e, i) => (
                <div
                  key={i}
                  style={{
                    fontSize: "0.85rem",
                    padding: "var(--space-2) 0",
                    borderTop: `1px solid var(--color-border)`,
                  }}
                >
                  <strong>{e.step}</strong> at {e.timestamp}
                  {e.error && (
                    <p style={{ color: "var(--color-danger)" }}>{e.error}</p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </section>
  );
}

export const Compliance = () => (
  <Stub title="Compliance" blurb="KYC, AML, velocity & case triage (Phase 3)." />
);
export const Ops = () => (
  <Stub title="Gov & Safety" blurb="Agent fleet, HITL approvals, incidents (Phase 4)." />
);
export const Evaluation = () => (
  <Stub title="Evaluation" blurb="Trace review, failure modes, judges (Phase 5)." />
);
export const Audit = () => (
  <Stub title="Audit" blurb="Read-only lineage & regulator lens (Phase 6)." />
);
