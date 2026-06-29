import { describe, it, expect } from "vitest";
import { canSee, visibleWorkspaces, WORKSPACES, type Role } from "./workspaces";

const byId = (id: string) => WORKSPACES.find((w) => w.id === id)!;

describe("role gating", () => {
  it("Assistant is open to any authenticated user (empty allowedRoles)", () => {
    expect(canSee(byId("assistant"), [])).toBe(true);
    expect(canSee(byId("assistant"), ["user"])).toBe(true);
  });

  it("a plain user sees Assistant + Commerce, but not Compliance/Ops/Eval/Audit", () => {
    const roles: Role[] = ["user"];
    const ids = visibleWorkspaces(roles).map((w) => w.id);
    expect(ids).toContain("assistant");
    expect(ids).toContain("commerce");
    expect(ids).not.toContain("compliance");
    expect(ids).not.toContain("ops");
    expect(ids).not.toContain("evaluation");
    expect(ids).not.toContain("audit");
  });

  it("an advisor adds Compliance + Evaluation but not Ops/Audit (admin-only)", () => {
    const ids = visibleWorkspaces(["advisor"]).map((w) => w.id);
    expect(ids).toEqual(
      expect.arrayContaining(["assistant", "commerce", "compliance", "evaluation"]),
    );
    expect(ids).not.toContain("ops");
    expect(ids).not.toContain("audit");
  });

  it("an admin sees every workspace", () => {
    const ids = visibleWorkspaces(["admin"]).map((w) => w.id);
    expect(ids).toEqual(WORKSPACES.map((w) => w.id));
  });

  it("preserves declared order", () => {
    const ids = visibleWorkspaces(["admin"]).map((w) => w.id);
    expect(ids).toEqual(["assistant", "commerce", "compliance", "ops", "evaluation", "audit"]);
  });

  it("multiple roles union their access", () => {
    const ids = visibleWorkspaces(["user", "advisor"]).map((w) => w.id);
    expect(ids).toContain("compliance");
    expect(ids).not.toContain("ops");
  });
});
