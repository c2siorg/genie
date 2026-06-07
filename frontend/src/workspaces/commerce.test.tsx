import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { Commerce } from "./pages";

// Mock hooks and components
vi.mock("../hooks/useCommerce", () => ({
  useOrder: vi.fn(() => ({
    order: null,
    loading: false,
    error: null,
    fetch: vi.fn(),
  })),
  useCreateOrder: vi.fn(() => ({
    create: vi.fn(async () => ({
      order_id: "ord-test",
      merchant_id: "m1",
      customer_id: "c1",
      items: [],
      total_paise: 100000,
      status: "pending",
      created_at: 1000,
    })),
    loading: false,
    error: null,
  })),
  useOrderAudit: vi.fn(() => ({
    entries: [],
    loading: false,
    error: null,
    fetch: vi.fn(),
  })),
}));

vi.mock("../provenance/provenance", () => ({
  useProvenance: vi.fn(() => ({
    open: vi.fn(),
  })),
}));

vi.mock("../components/Money", () => ({
  Money: ({ paise, intent }: { paise: number; intent?: string }) => (
    <span data-paise={paise} data-intent={intent}>
      ₹{(paise / 100).toFixed(2)}
    </span>
  ),
}));

vi.mock("../components/StateMachine", () => ({
  StateMachine: ({ current }: { current: string }) => (
    <div data-status={current}>Status: {current}</div>
  ),
}));

describe("Commerce workspace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the Commerce workspace heading", () => {
    render(<Commerce />);
    expect(screen.getByRole("heading", { name: "Commerce" })).toBeInTheDocument();
  });

  it("shows the create demo order button when no order is present", () => {
    render(<Commerce />);
    expect(screen.getByRole("button", { name: /Create demo order/i })).toBeInTheDocument();
  });

  it("shows the subtitle mentioning Phase 2 API integration", () => {
    render(<Commerce />);
    expect(screen.getByText(/Phase 2 real API integration/i)).toBeInTheDocument();
  });

  it("does not show loading text when initially rendered", () => {
    render(<Commerce />);
    expect(screen.queryByText(/Loading order/i)).not.toBeInTheDocument();
  });

  it("shows create button is not disabled on initial render", () => {
    render(<Commerce />);
    const button = screen.getByRole("button", { name: /Create demo order/i });
    expect(button).not.toBeDisabled();
  });

  it("exports the Commerce component", () => {
    // Verify the component is properly exported
    expect(Commerce).toBeDefined();
    expect(typeof Commerce).toBe("function");
  });
});
