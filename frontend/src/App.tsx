import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./shell/AppShell";
import {
  Assistant,
  Audit,
  Commerce,
  Compliance,
  Evaluation,
  Ops,
} from "./workspaces/pages";

/**
 * App wires the role-gated shell and workspace routes. The shell itself
 * decides which nav links to show per the user's roles (workspaces.ts);
 * these routes mount the corresponding pages. Deep, per-workspace routing
 * (e.g. /commerce/orders/:id) is added as each slice is built (Phases 2-6).
 */
export default function App() {
  return (
    <Routes>
      <Route path="/" element={<AppShell />}>
        <Route index element={<Navigate to="/assistant" replace />} />
        <Route path="assistant" element={<Assistant />} />
        <Route path="commerce" element={<Commerce />} />
        <Route path="compliance" element={<Compliance />} />
        <Route path="ops" element={<Ops />} />
        <Route path="evaluation" element={<Evaluation />} />
        <Route path="audit" element={<Audit />} />
        <Route path="*" element={<Navigate to="/assistant" replace />} />
      </Route>
    </Routes>
  );
}
