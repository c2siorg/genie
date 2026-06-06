import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StateMachine, statusForStep, type Step } from "./StateMachine";

const STEPS: Step[] = [
  { key: "created", label: "Created" },
  { key: "payment", label: "Payment" },
  { key: "settlement", label: "Settlement" },
  { key: "fulfilled", label: "Fulfilled" },
];

describe("statusForStep", () => {
  it("marks steps before current done, current current, after pending", () => {
    expect(statusForStep(STEPS, "settlement", undefined, 0)).toBe("done");
    expect(statusForStep(STEPS, "settlement", undefined, 1)).toBe("done");
    expect(statusForStep(STEPS, "settlement", undefined, 2)).toBe("current");
    expect(statusForStep(STEPS, "settlement", undefined, 3)).toBe("pending");
  });

  it("blocked overrides and flags the blocked step", () => {
    expect(statusForStep(STEPS, "settlement", "settlement", 2)).toBe("blocked");
  });

  it("unknown current key leaves all steps pending", () => {
    expect(statusForStep(STEPS, "nope", undefined, 0)).toBe("pending");
  });
});

describe("<StateMachine>", () => {
  it("renders each step with its derived status data attribute", () => {
    render(<StateMachine steps={STEPS} current="settlement" />);
    expect(screen.getByText("Created").closest("li")).toHaveAttribute(
      "data-status",
      "done",
    );
    expect(screen.getByText("Settlement").closest("li")).toHaveAttribute(
      "data-status",
      "current",
    );
    expect(screen.getByText("Fulfilled").closest("li")).toHaveAttribute(
      "data-status",
      "pending",
    );
  });

  it("marks the current step with aria-current for assistive tech", () => {
    render(<StateMachine steps={STEPS} current="payment" />);
    expect(screen.getByText("Payment").closest("li")).toHaveAttribute(
      "aria-current",
      "step",
    );
  });

  it("renders a blocked step distinctly", () => {
    render(<StateMachine steps={STEPS} current="settlement" blocked="settlement" />);
    expect(screen.getByText("Settlement").closest("li")).toHaveAttribute(
      "data-status",
      "blocked",
    );
  });

  it("renders optional step detail (e.g. timestamp/amount)", () => {
    render(
      <StateMachine
        steps={[{ key: "a", label: "Created", detail: "12:01" }]}
        current="a"
      />,
    );
    expect(screen.getByText("12:01")).toBeInTheDocument();
  });
});
