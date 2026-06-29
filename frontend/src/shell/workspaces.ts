// workspaces.ts — the five product workspaces (plan.md §2) plus the Audit lens,
// and the role-gating that decides which a user may see.
//
// IMPORTANT (grounded in reality): the backend currently defines only three
// roles — "user", "advisor", "admin" (pkg/auth/types.go). We gate against
// THOSE, not invented roles. Finer-grained roles (e.g. compliance_officer,
// merchant) are a backend change; when they exist, extend `allowedRoles` here.

export type Role = "user" | "advisor" | "admin";

export interface Workspace {
  id: string;
  /** Route path under the /ui/app/ basename. */
  path: string;
  label: string;
  /** Short description for the nav / launcher. */
  blurb: string;
  /** Roles permitted to see this workspace. Empty = any authenticated user. */
  allowedRoles: Role[];
}

export const WORKSPACES: Workspace[] = [
  {
    id: "assistant",
    path: "/assistant",
    label: "Assistant",
    blurb: "Conversational financial assistant",
    allowedRoles: [], // any authenticated user
  },
  {
    id: "commerce",
    path: "/commerce",
    label: "Commerce",
    blurb: "Orders, payments, settlement & payouts",
    allowedRoles: ["user", "advisor", "admin"],
  },
  {
    id: "compliance",
    path: "/compliance",
    label: "Compliance",
    blurb: "KYC, AML, velocity & case triage",
    allowedRoles: ["advisor", "admin"],
  },
  {
    id: "ops",
    path: "/ops",
    label: "Gov & Safety",
    blurb: "Agent fleet, HITL approvals, incidents",
    allowedRoles: ["admin"],
  },
  {
    id: "evaluation",
    path: "/evaluation",
    label: "Evaluation",
    blurb: "Trace review, failure modes, judges",
    allowedRoles: ["advisor", "admin"],
  },
  {
    id: "audit",
    path: "/audit",
    label: "Audit",
    blurb: "Read-only lineage & regulator lens",
    allowedRoles: ["admin"],
  },
];

/** canSee reports whether a user with `roles` may access `ws`. */
export function canSee(ws: Workspace, roles: readonly Role[]): boolean {
  if (ws.allowedRoles.length === 0) return true; // open to any authenticated user
  return ws.allowedRoles.some((r) => roles.includes(r));
}

/** visibleWorkspaces returns, in order, the workspaces a user may access. */
export function visibleWorkspaces(roles: readonly Role[]): Workspace[] {
  return WORKSPACES.filter((ws) => canSee(ws, roles));
}
