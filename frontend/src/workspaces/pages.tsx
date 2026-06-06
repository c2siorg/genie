// Workspace pages. Most are Phase 2-6 stubs; Commerce is wired up enough to
// demonstrate the shared primitives working together (Money + StateMachine +
// the provenance drawer), proving the Phase 1 foundation end-to-end.

import { Money } from "../components/Money";
import { StateMachine, type Step } from "../components/StateMachine";
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

const SETTLEMENT_STEPS: Step[] = [
  { key: "created", label: "Created", detail: "12:00" },
  { key: "payment_initiated", label: "Payment", detail: "12:01" },
  { key: "payment_confirmed", label: "Confirmed", detail: "12:01" },
  { key: "settlement", label: "Settlement", detail: "12:02" },
  { key: "fulfilled", label: "Fulfilled" },
];

export function Commerce() {
  const { open } = useProvenance();
  return (
    <section>
      <h1>Commerce</h1>
      <p style={{ color: "var(--color-text-muted)" }}>
        Orders, payments, settlement &amp; payouts (full build in Phase 2).
      </p>
      <div
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius)",
          padding: "var(--space-6)",
          marginTop: "var(--space-4)",
        }}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <strong>Order ord-1001</strong>
          <Money paise={10000000} intent="credit" />
        </div>
        <div style={{ margin: "var(--space-4) 0" }}>
          <StateMachine
            steps={SETTLEMENT_STEPS}
            current="settlement"
            aria-label="Settlement status for ord-1001"
          />
        </div>
        <button onClick={() => open("ord-1001")}>Why? · View provenance ↗</button>
      </div>
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
