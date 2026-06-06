// session.tsx — authentication/session context.
//
// The session is cookie-based (HttpOnly), so on mount we ask the server who we
// are via GET /auth/me. There is no token in JS to persist. login()/logout()
// hit the real auth endpoints; apiFetch captures the rotated CSRF token from
// the response header automatically.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { apiFetch, ApiError } from "../lib/api";
import type { Role } from "../shell/workspaces";

export interface User {
  id: string;
  email: string;
  name?: string;
  roles: Role[];
}

interface SessionState {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

const Ctx = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const me = await apiFetch<User>("/auth/me");
      setUser(me);
    } catch (e) {
      if (e instanceof ApiError && e.isUnauthenticated) setUser(null);
      else setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const resp = await apiFetch<{ user: User }>("/auth/login", {
      method: "POST",
      json: { email, password },
    });
    setUser(resp.user);
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiFetch("/auth/logout", { method: "POST" });
    } finally {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <Ctx.Provider value={{ user, loading, login, logout, refresh }}>
      {children}
    </Ctx.Provider>
  );
}

export function useSession(): SessionState {
  const v = useContext(Ctx);
  if (!v) throw new Error("useSession must be used within a SessionProvider");
  return v;
}
