import { type ReactNode } from "react";
import { BrowserRouter, NavLink, Navigate, Outlet, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./auth/AuthProvider";
import { SCOPE_ADMIN, SCOPE_AUDIT } from "./auth/scopes";
import { AuditPage } from "./pages/AuditPage";
import { FiltersPage } from "./pages/FiltersPage";
import { LoginPage } from "./pages/LoginPage";
import { MessageDetailPage } from "./pages/MessageDetailPage";
import { MessagesPage } from "./pages/MessagesPage";
import { ResetPage } from "./pages/ResetPage";
import { StatusPage } from "./pages/StatusPage";
import { navItems } from "./ui/nav";

function SkipLink() {
  return (
    <a className="skip-link" href="#app-main">
      Skip to main content
    </a>
  );
}

function NavItem({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink to={to} className={({ isActive }) => (isActive ? "nav-active" : undefined)} end={to === "/"}>
      {children}
    </NavLink>
  );
}

function Shell() {
  const { state, hasScope, logout } = useAuth();
  const signedIn = state.status === "signed_in";
  const items = signedIn
    ? navItems({ canAudit: hasScope(SCOPE_AUDIT), canReset: hasScope(SCOPE_ADMIN) })
    : [];
  return (
    <div className="app">
      <SkipLink />
      <header className="topbar">
        <NavLink className="brand" to="/">
          LabSyslog
        </NavLink>
        <nav aria-label="Primary">
          {items.map((item) => (
            <NavItem key={item.to} to={item.to}>
              {item.label}
            </NavItem>
          ))}
          {signedIn ? (
            <button type="button" className="linkish" onClick={() => void logout()}>
              Sign out
            </button>
          ) : null}
        </nav>
      </header>
      <div id="app-main">
        <Outlet />
      </div>
    </div>
  );
}

function RequireSession() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status !== "signed_in") {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}

function RedirectIfSignedIn() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status === "signed_in") {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route element={<Shell />}>
            <Route element={<RedirectIfSignedIn />}>
              <Route path="/login" element={<LoginPage />} />
            </Route>
            <Route element={<RequireSession />}>
              <Route path="/" element={<MessagesPage />} />
              <Route path="/messages/:id" element={<MessageDetailPage />} />
              <Route path="/status" element={<StatusPage />} />
              <Route path="/filters" element={<FiltersPage />} />
              <Route path="/audit" element={<AuditPage />} />
              <Route path="/reset" element={<ResetPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
