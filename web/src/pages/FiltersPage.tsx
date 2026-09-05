import { useCallback, useEffect, useState } from "react";
import { APIError, applyFilters, getState } from "../api/client";
import type { Filter } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_ADMIN } from "../auth/scopes";

export function FiltersPage() {
  const { hasScope } = useAuth();
  const canAdmin = hasScope(SCOPE_ADMIN);
  const [items, setItems] = useState<Filter[] | null>(null);
  const [revision, setRevision] = useState("");
  const [error, setError] = useState("");
  const [busyName, setBusyName] = useState("");

  const reload = useCallback(async () => {
    const state = await getState();
    setItems(state.spec.filters ?? []);
    setRevision(state.revision);
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await reload();
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load filters.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [reload]);

  async function onToggle(filter: Filter, enabled: boolean) {
    if (!canAdmin || items === null) {
      return;
    }
    setBusyName(filter.name);
    setError("");
    try {
      const next = items.map((f) => (f.name === filter.name ? { ...f, enabled } : f));
      const reason = enabled ? "ui: enable filter" : "ui: disable filter";
      await applyFilters(revision, next, reason);
      await reload();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Could not update filter.");
    } finally {
      setBusyName("");
    }
  }

  if (items === null && error === "") {
    return (
      <main className="page">
        <p role="status">Loading filters…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Filters</h1>
      <p>
        First enabled match wins. Unmatched messages are captured. Mutations go through plan/apply
        <code> replaceFilters</code>; there is no parallel CRUD and no YAML form.
      </p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <table className="data">
        <caption>Snapshot filters. Enable/disable is syslog.admin via replaceFilters.</caption>
        <thead>
          <tr>
            <th>Name</th>
            <th>Enabled</th>
            <th>Match</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          {(items ?? []).length === 0 ? (
            <tr>
              <td colSpan={4} className="muted">
                No filters. Unmatched traffic is captured.
              </td>
            </tr>
          ) : (
            (items ?? []).map((f) => (
              <tr key={f.name}>
                <td>
                  <code>{f.name}</code>
                </td>
                <td>
                  <label>
                    <input
                      type="checkbox"
                      checked={f.enabled !== false}
                      disabled={!canAdmin || busyName === f.name}
                      aria-label={`Enable ${f.name}`}
                      onChange={(e) => void onToggle(f, e.target.checked)}
                    />
                  </label>
                </td>
                <td>{formatMatch(f)}</td>
                <td>
                  {f.action.mode}
                  {f.action.tag ? ` (${f.action.tag})` : ""}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </main>
  );
}

function formatMatch(f: Filter): string {
  const parts: string[] = [];
  const m = f.match;
  if (m.sourceCidrs?.length) parts.push(`cidrs=${m.sourceCidrs.join(",")}`);
  if (m.facilities?.length) parts.push(`facilities=${m.facilities.join(",")}`);
  if (m.severities?.length) parts.push(`severities=${m.severities.join(",")}`);
  if (m.severityAtLeast) parts.push(`severityAtLeast=${m.severityAtLeast}`);
  if (m.appNames?.length) parts.push(`app=${m.appNames.join(",")}`);
  if (m.hostnames?.length) parts.push(`host=${m.hostnames.join(",")}`);
  if (m.transports?.length) parts.push(`transport=${m.transports.join(",")}`);
  return parts.join(" ") || "(any)";
}
