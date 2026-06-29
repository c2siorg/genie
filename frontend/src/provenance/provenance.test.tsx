import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { ProvenanceDrawer } from "./provenance";
import * as api from "../lib/api";

const ENTRIES = {
  total: 2,
  entries: [
    {
      id: "l1",
      timestamp: "2026-06-06T12:00:00Z",
      user_id: "u1",
      resource_id: "ord-1",
      resource_type: "order",
      action: "order_created",
      decision: "allow",
      reason_code: "OK",
      agent_id: "merchant_supervisor",
    },
    {
      id: "l2",
      timestamp: "2026-06-06T12:01:00Z",
      user_id: "u1",
      resource_id: "ord-1",
      resource_type: "order",
      action: "settlement_completed",
      decision: "allow",
      reason_code: "RECONCILED",
      agent_id: "settlement_supervisor",
    },
  ],
};

describe("<ProvenanceDrawer>", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("renders nothing when no resource is selected", () => {
    const { container } = render(
      <ProvenanceDrawer resourceId={null} onClose={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("queries the real POST lineage endpoints with the resource id", async () => {
    const spy = vi.spyOn(api, "apiFetch").mockImplementation(async (path: string) => {
      if (path.startsWith("/lineage/query")) return ENTRIES as never;
      return { valid: true, total_entries: 2 } as never;
    });
    render(<ProvenanceDrawer resourceId="ord-1" onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText(/Hash chain intact/)).toBeInTheDocument());

    expect(spy).toHaveBeenCalledWith("/lineage/query?resource_id=ord-1", {
      method: "POST",
    });
    expect(spy).toHaveBeenCalledWith("/lineage/verify?resource_id=ord-1", {
      method: "POST",
    });
  });

  it("renders the agent chain (who did what)", async () => {
    vi.spyOn(api, "apiFetch").mockImplementation(async (path: string) =>
      (path.startsWith("/lineage/query")
        ? ENTRIES
        : { valid: true, total_entries: 2 }) as never,
    );
    render(<ProvenanceDrawer resourceId="ord-1" onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText("order_created")).toBeInTheDocument());
    expect(screen.getByText(/merchant_supervisor/)).toBeInTheDocument();
    expect(screen.getByText("settlement_completed")).toBeInTheDocument();
    expect(screen.getByText(/settlement_supervisor/)).toBeInTheDocument();
  });

  it("flags a broken hash chain prominently", async () => {
    vi.spyOn(api, "apiFetch").mockImplementation(async (path: string) =>
      (path.startsWith("/lineage/query")
        ? { total: 0, entries: [] }
        : { valid: false, total_entries: 3, broken_at: "l2" }) as never,
    );
    render(<ProvenanceDrawer resourceId="ord-x" onClose={() => {}} />);
    await waitFor(() =>
      expect(screen.getByText(/Hash chain BROKEN/)).toBeInTheDocument(),
    );
    expect(screen.getByText(/at l2/)).toBeInTheDocument();
  });

  it("surfaces an error without crashing", async () => {
    vi.spyOn(api, "apiFetch").mockRejectedValue(new api.ApiError(401, "unauthenticated"));
    render(<ProvenanceDrawer resourceId="ord-1" onClose={() => {}} />);
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(/unauthenticated/),
    );
  });
});
