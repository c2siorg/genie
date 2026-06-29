// AppShell — the role-gated workspace shell (plan.md §4). Renders the top bar,
// the workspace navigation (only the workspaces the user's roles permit), and
// the active workspace via <Outlet/>. Wrapped in the ProvenanceProvider so any
// workspace can open the provenance drawer.

import { NavLink, Outlet } from "react-router-dom";
import { useSession } from "../auth/session";
import { visibleWorkspaces } from "./workspaces";
import { ProvenanceProvider } from "../provenance/provenance";

export function AppShell() {
  const { user, logout } = useSession();
  const roles = user?.roles ?? [];
  const workspaces = visibleWorkspaces(roles);

  return (
    <ProvenanceProvider>
      <div className="shell">
        <header className="shell__topbar">
          <div className="shell__brand">
            <span aria-hidden="true">🧞</span> <strong>Genie</strong>
          </div>
          <nav className="shell__nav" aria-label="Workspaces">
            {workspaces.map((ws) => (
              <NavLink
                key={ws.id}
                to={ws.path}
                className={({ isActive }) =>
                  "shell__navlink" + (isActive ? " shell__navlink--active" : "")
                }
                title={ws.blurb}
              >
                {ws.label}
              </NavLink>
            ))}
          </nav>
          <div className="shell__user">
            {user ? (
              <>
                <span className="shell__email">{user.email}</span>
                <button className="shell__logout" onClick={() => void logout()}>
                  Logout
                </button>
              </>
            ) : (
              <span className="shell__email">Not signed in</span>
            )}
          </div>
        </header>
        <main className="shell__main">
          <Outlet />
        </main>
      </div>
    </ProvenanceProvider>
  );
}
