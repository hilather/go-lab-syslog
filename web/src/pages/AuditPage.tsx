import { useEffect, useState } from "react";
import { APIError, listAudit } from "../api/client";
import type { AuditEvent } from "../api/types";

export function AuditPage() {
  const [items, setItems] = useState<AuditEvent[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listAudit();
        if (!cancelled) {
          setItems(list.items ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load audit.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (error !== "") {
    return (
      <main className="page">
        <p className="banner-error" role="alert">
          {error}
        </p>
      </main>
    );
  }
  if (items === null) {
    return (
      <main className="page">
        <p role="status">Loading audit…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Audit</h1>
      <p>Recent management mutations. Ingest is not audited. Reset wipes this ring with the store.</p>
      <table className="data">
        <caption>Newest first.</caption>
        <thead>
          <tr>
            <th>At</th>
            <th>Actor</th>
            <th>Operation</th>
            <th>Reason</th>
            <th>Revision</th>
          </tr>
        </thead>
        <tbody>
          {items.length === 0 ? (
            <tr>
              <td colSpan={5} className="muted">
                No audit events.
              </td>
            </tr>
          ) : (
            items.map((e) => (
              <tr key={e.id}>
                <td>{e.at}</td>
                <td>{e.actor}</td>
                <td>{e.operation}</td>
                <td>{e.reason || "—"}</td>
                <td>
                  <code>{e.revision}</code>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </main>
  );
}
