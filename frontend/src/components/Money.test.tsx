import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Money } from "./Money";

describe("<Money>", () => {
  it("renders grouped rupees from paise", () => {
    render(<Money paise={10000000} />);
    expect(screen.getByText("₹1,00,000.00")).toBeInTheDocument();
  });

  it("exposes raw paise on a data attribute for automation", () => {
    render(<Money paise={12345} />);
    expect(screen.getByText("₹123.45")).toHaveAttribute("data-paise", "12345");
  });

  it("marks intent without inline color (theming via tokens)", () => {
    render(<Money paise={500} intent="credit" />);
    const el = screen.getByText("₹5.00");
    expect(el).toHaveClass("money", "money--credit");
    expect(el).toHaveAttribute("data-intent", "credit");
  });

  it("shows an explicit sign when requested (e.g. ledger deltas)", () => {
    render(<Money paise={500} showSign />);
    expect(screen.getByText("+₹5.00")).toBeInTheDocument();
  });

  it("renders refunds (negative) unambiguously", () => {
    render(<Money paise={-500} intent="debit" />);
    expect(screen.getByText("-₹5.00")).toBeInTheDocument();
  });

  it("provides an accessible label equal to the rendered amount", () => {
    render(<Money paise={100} />);
    expect(screen.getByLabelText("₹1.00")).toBeInTheDocument();
  });
});
