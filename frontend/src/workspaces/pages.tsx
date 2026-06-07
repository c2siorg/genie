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
import { useHITLApprovals, useComplianceCheck, useComplianceCheckResult } from "../hooks/useCompliance";
import { useAgentFleet, useIncidents, useKillSwitch } from "../hooks/useGovernance";

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

export function Compliance() {
  const { approvals, loading: loadingApprovals, error: approvalsError, fetch: fetchApprovals } =
    useHITLApprovals();
  const { check, checking: checkingPayment } = useComplianceCheck();
  const [lastCheckId, setLastCheckId] = useState<string | null>(null);
  const {
    result: checkResult,
    loading: loadingCheckResult,
    error: checkResultError,
    fetch: fetchCheckResult,
  } = useComplianceCheckResult(lastCheckId);

  const handleCheckPayment = async () => {
    const result = await check("demo-payment", "account-from", "account-to", 100000); // ₹1000
    if (result) {
      setLastCheckId(result.check_id);
    }
  };

  useEffect(() => {
    fetchApprovals();
  }, [fetchApprovals]);

  useEffect(() => {
    if (lastCheckId) {
      fetchCheckResult();
    }
  }, [lastCheckId, fetchCheckResult]);

  const getRiskColor = (score: number) => {
    if (score < 30) return "var(--color-success)"; // Low risk
    if (score < 60) return "var(--color-warn)"; // Medium risk
    return "var(--color-danger)"; // High risk
  };

  return (
    <section>
      <h1>Compliance</h1>
      <p style={{ color: "var(--color-text-muted)" }}>
        KYC onboarding, payment monitoring, AML screening, HITL approvals (Phase 3 real integration).
      </p>

      {/* Section 1: Payment Compliance Check */}
      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius)",
          padding: "var(--space-6)",
          marginTop: "var(--space-6)",
        }}
      >
        <h2 style={{ marginTop: 0 }}>Real-Time Payment Compliance</h2>
        <p style={{ color: "var(--color-text-muted)" }}>
          Initiate an async compliance check with AML screening, velocity limits, and fraud detection.
        </p>

        <button
          onClick={handleCheckPayment}
          disabled={checkingPayment}
          style={{
            padding: "var(--space-2) var(--space-4)",
            background: "var(--color-info)",
            color: "white",
            border: "none",
            borderRadius: "var(--radius)",
            cursor: "pointer",
          }}
        >
          {checkingPayment ? "Checking..." : "Check payment compliance"}
        </button>

        {checkResultError && (
          <p style={{ color: "var(--color-danger)", marginTop: "var(--space-2)" }}>
            Error: {checkResultError}
          </p>
        )}

        {checkResult && (
          <div style={{ marginTop: "var(--space-4)" }}>
            <h3>Check Result</h3>
            <div style={{ fontSize: "0.9rem" }}>
              <p>
                <strong>Decision:</strong>{" "}
                <span style={{ color: getRiskColor(checkResult.fraud_score) }}>
                  {checkResult.decision.toUpperCase()}
                </span>
              </p>
              <p>
                <strong>AML Result:</strong> {checkResult.aml_result} {checkResult.aml_reason && `(${checkResult.aml_reason})`}
              </p>
              <p>
                <strong>Velocity:</strong> {checkResult.velocity_result}
                {checkResult.velocity_reason && ` – ${checkResult.velocity_reason}`}
              </p>
              <p>
                <strong>Fraud Score:</strong>{" "}
                <span style={{ color: getRiskColor(checkResult.fraud_score) }}>
                  {checkResult.fraud_score}/100
                </span>
              </p>
              {checkResult.detected_patterns.length > 0 && (
                <p>
                  <strong>Detected Patterns:</strong> {checkResult.detected_patterns.join(", ")}
                </p>
              )}
              <p style={{ fontSize: "0.85rem", color: "var(--color-text-muted)" }}>
                Checked at: {new Date(checkResult.checked_at).toLocaleString()}
              </p>
            </div>
          </div>
        )}

        {loadingCheckResult && (
          <p style={{ color: "var(--color-text-muted)", marginTop: "var(--space-4)" }}>
            Fetching check result...
          </p>
        )}
      </div>

      {/* Section 2: HITL Approval Queue */}
      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius)",
          padding: "var(--space-6)",
          marginTop: "var(--space-6)",
        }}
      >
        <h2 style={{ marginTop: 0 }}>Human-in-the-Loop (HITL) Approval Queue</h2>
        <p style={{ color: "var(--color-text-muted)" }}>
          Flagged KYC applications and payments requiring manual compliance review.
        </p>

        {approvalsError && (
          <p style={{ color: "var(--color-danger)" }}>Error: {approvalsError}</p>
        )}

        {loadingApprovals ? (
          <p style={{ color: "var(--color-text-muted)" }}>Loading approvals...</p>
        ) : approvals.length === 0 ? (
          <p style={{ color: "var(--color-text-muted)" }}>No pending approvals.</p>
        ) : (
          <div>
            <p style={{ color: "var(--color-text-muted)", fontSize: "0.9rem" }}>
              {approvals.length} pending approval{approvals.length !== 1 ? "s" : ""}
            </p>
            {approvals.map((approval, i) => (
              <div
                key={i}
                style={{
                  padding: "var(--space-3)",
                  borderTop: i === 0 ? "none" : "1px solid var(--color-border)",
                  fontSize: "0.9rem",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "var(--space-2)" }}>
                  <strong>
                    {approval.request_type.toUpperCase()}: {approval.entity_id}
                  </strong>
                  <span style={{ color: "var(--color-text-muted)", fontSize: "0.85rem" }}>
                    {new Date(approval.created_at).toLocaleString()}
                  </span>
                </div>
                <p style={{ color: "var(--color-text-muted)", margin: 0 }}>{approval.reason}</p>
                {approval.risk_factors && approval.risk_factors.length > 0 && (
                  <p style={{ color: "var(--color-warn)", margin: "var(--space-1) 0 0 0", fontSize: "0.85rem" }}>
                    Risk: {approval.risk_factors.join(", ")}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
export function Ops() {
  const { agents, loading: loadingAgents, error: agentsError, fetch: fetchAgents } =
    useAgentFleet();
  const { incidents, loading: loadingIncidents, error: incidentsError, fetch: fetchIncidents } =
    useIncidents(10);
  const { status: killSwitchStatus, loading: loadingKillSwitch, fetch: fetchKillSwitch } =
    useKillSwitch();

  useEffect(() => {
    fetchAgents();
    fetchIncidents();
    fetchKillSwitch();
  }, [fetchAgents, fetchIncidents, fetchKillSwitch]);

  const getRingColor = (ring: string) => {
    switch (ring) {
      case "admin":
        return "var(--color-danger)";
      case "standard":
        return "var(--color-success)";
      case "restricted":
        return "var(--color-warn)";
      case "sandboxed":
        return "var(--color-info)";
      default:
        return "var(--color-text-muted)";
    }
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case "low":
        return "var(--color-success)";
      case "moderate":
        return "var(--color-warn)";
      case "high":
        return "var(--color-danger)";
      default:
        return "var(--color-text-muted)";
    }
  };

  return (
    <section>
      <h1>Governance & Safety</h1>
      <p style={{ color: "var(--color-text-muted)" }}>
        Agent fleet monitoring, incident response, policy enforcement, and emergency controls (Phase 4).
      </p>

      {/* Section 1: Agent Fleet Dashboard */}
      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius)",
          padding: "var(--space-6)",
          marginTop: "var(--space-6)",
        }}
      >
        <h2 style={{ marginTop: 0 }}>Agent Fleet Health</h2>
        <p style={{ color: "var(--color-text-muted)" }}>
          Real-time monitoring of all agents with trust scores and ring assignments.
        </p>

        {agentsError && <p style={{ color: "var(--color-danger)" }}>Error: {agentsError}</p>}

        {loadingAgents ? (
          <p style={{ color: "var(--color-text-muted)" }}>Loading agents...</p>
        ) : agents.length === 0 ? (
          <p style={{ color: "var(--color-text-muted)" }}>No agents found.</p>
        ) : (
          <div>
            <p style={{ color: "var(--color-text-muted)", fontSize: "0.9rem" }}>
              {agents.length} agents total
            </p>
            {agents.map((agent, i) => (
              <div
                key={i}
                style={{
                  padding: "var(--space-3)",
                  borderTop: i === 0 ? "none" : "1px solid var(--color-border)",
                  fontSize: "0.9rem",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "var(--space-2)" }}>
                  <div>
                    <strong>{agent.name}</strong>
                    <p style={{ color: "var(--color-text-muted)", margin: "var(--space-1) 0 0 0" }}>
                      {agent.agent_id}
                    </p>
                  </div>
                  <div style={{ textAlign: "right" }}>
                    <div style={{ color: getRingColor(agent.ring), fontWeight: "bold" }}>
                      {agent.ring.toUpperCase()}
                    </div>
                    <div style={{ color: "var(--color-text-muted)", fontSize: "0.85rem", marginTop: "var(--space-1)" }}>
                      {agent.status.toUpperCase()}
                    </div>
                  </div>
                </div>
                <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.85rem" }}>
                  <span>Trust Score: {agent.trust_score}/100</span>
                  <span>{agent.capabilities.join(", ")}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Section 2: Incident Tracking */}
      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius)",
          padding: "var(--space-6)",
          marginTop: "var(--space-6)",
        }}
      >
        <h2 style={{ marginTop: 0 }}>Incident Response</h2>
        <p style={{ color: "var(--color-text-muted)" }}>
          Recent incidents with severity, failure modes, and resolution status.
        </p>

        {incidentsError && <p style={{ color: "var(--color-danger)" }}>Error: {incidentsError}</p>}

        {loadingIncidents ? (
          <p style={{ color: "var(--color-text-muted)" }}>Loading incidents...</p>
        ) : incidents.length === 0 ? (
          <p style={{ color: "var(--color-success)" }}>No recent incidents. 🎉</p>
        ) : (
          <div>
            {incidents.map((incident, i) => (
              <div
                key={i}
                style={{
                  padding: "var(--space-3)",
                  borderTop: i === 0 ? "none" : "1px solid var(--color-border)",
                  fontSize: "0.9rem",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "var(--space-2)" }}>
                  <div>
                    <strong>{incident.use_case}</strong>
                    <p style={{ color: "var(--color-text-muted)", margin: "var(--space-1) 0 0 0" }}>
                      {incident.description}
                    </p>
                  </div>
                  <div style={{ textAlign: "right" }}>
                    <div style={{ color: getSeverityColor(incident.severity), fontWeight: "bold" }}>
                      {incident.severity.toUpperCase()}
                    </div>
                    <div
                      style={{
                        color: incident.status === "ongoing" ? "var(--color-warn)" : "var(--color-success)",
                        fontSize: "0.85rem",
                        marginTop: "var(--space-1)",
                      }}
                    >
                      {incident.status.toUpperCase()}
                    </div>
                  </div>
                </div>
                <div style={{ fontSize: "0.85rem", color: "var(--color-text-muted)" }}>
                  <p style={{ margin: 0 }}>
                    <strong>Failure Mode:</strong> {incident.failure_mode}
                  </p>
                  <p style={{ margin: "var(--space-1) 0 0 0" }}>
                    <strong>Detected:</strong> {new Date(incident.detected_at).toLocaleString()}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Section 3: Kill Switch Controls */}
      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius)",
          padding: "var(--space-6)",
          marginTop: "var(--space-6)",
        }}
      >
        <h2 style={{ marginTop: 0 }}>Emergency Controls</h2>
        <p style={{ color: "var(--color-text-muted)" }}>
          Global kill-switch status. Use only in emergencies to halt all agents.
        </p>

        {loadingKillSwitch ? (
          <p style={{ color: "var(--color-text-muted)" }}>Loading kill-switch status...</p>
        ) : killSwitchStatus ? (
          <div>
            <div
              style={{
                padding: "var(--space-3)",
                background: killSwitchStatus.is_active
                  ? "var(--color-danger)"
                  : "var(--color-success)",
                color: "white",
                borderRadius: "var(--radius)",
                marginBottom: "var(--space-4)",
                textAlign: "center",
              }}
            >
              <strong>
                {killSwitchStatus.is_active ? "🛑 KILL SWITCH ACTIVE" : "✓ All systems operational"}
              </strong>
            </div>

            {killSwitchStatus.activations.length > 0 && (
              <div>
                <p style={{ color: "var(--color-text-muted)", fontSize: "0.9rem" }}>
                  {killSwitchStatus.activations.length} activation{killSwitchStatus.activations.length !== 1 ? "s" : ""}
                </p>
                {killSwitchStatus.activations.slice(0, 3).map((activation, i) => (
                  <div
                    key={i}
                    style={{
                      padding: "var(--space-2)",
                      borderTop: i === 0 ? "none" : "1px solid var(--color-border)",
                      fontSize: "0.85rem",
                    }}
                  >
                    <p style={{ margin: 0 }}>
                      <strong>{activation.scope.toUpperCase()}</strong> {activation.target && `(${activation.target})`}
                    </p>
                    <p style={{ color: "var(--color-text-muted)", margin: "var(--space-1) 0 0 0" }}>
                      {activation.reason}
                    </p>
                    <p style={{ color: "var(--color-text-muted)", fontSize: "0.75rem", margin: "var(--space-1) 0 0 0" }}>
                      {new Date(activation.activated_at).toLocaleString()}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : null}
      </div>
    </section>
  );
}
export const Evaluation = () => (
  <Stub title="Evaluation" blurb="Trace review, failure modes, judges (Phase 5)." />
);
export const Audit = () => (
  <Stub title="Audit" blurb="Read-only lineage & regulator lens (Phase 6)." />
);
