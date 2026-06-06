// StateMachine — a reusable horizontal timeline of workflow states
// (plan.md §3.3: "state is the story"). Used by settlement, compliance
// decisions, HITL, etc. It renders an ordered set of steps, highlights the
// current one, marks completed steps done, and flags a blocked/failed step.

export type StepStatus = "done" | "current" | "blocked" | "pending";

export interface Step {
  key: string;
  label: string;
  /** Optional sub-label, e.g. a timestamp or amount. */
  detail?: string;
}

export interface StateMachineProps {
  steps: Step[];
  /** The key of the step the workflow is currently at. */
  current: string;
  /** If set, this step is marked blocked/failed (overrides current styling). */
  blocked?: string;
  "aria-label"?: string;
}

/** Derive each step's status from the ordered list, current, and blocked keys. */
export function statusForStep(
  steps: Step[],
  current: string,
  blocked: string | undefined,
  index: number,
): StepStatus {
  if (blocked && steps[index].key === blocked) return "blocked";
  const currentIndex = steps.findIndex((s) => s.key === current);
  if (currentIndex === -1) return "pending";
  if (index < currentIndex) return "done";
  if (index === currentIndex) return "current";
  return "pending";
}

export function StateMachine({
  steps,
  current,
  blocked,
  "aria-label": ariaLabel = "workflow status",
}: StateMachineProps) {
  return (
    <ol className="statemachine" aria-label={ariaLabel}>
      {steps.map((step, i) => {
        const status = statusForStep(steps, current, blocked, i);
        return (
          <li
            key={step.key}
            className={`statemachine__step statemachine__step--${status}`}
            data-status={status}
            aria-current={status === "current" ? "step" : undefined}
          >
            <span className="statemachine__label">{step.label}</span>
            {step.detail && (
              <span className="statemachine__detail">{step.detail}</span>
            )}
          </li>
        );
      })}
    </ol>
  );
}
