import { useEffect, useState } from "react";
import { APIError, getState, getStats, getStatus } from "../api/client";
import type { StateView, Stats, Status } from "../api/types";

export function StatusPage() {
  const [status, setStatus] = useState<Status | null>(null);
  const [state, setState] = useState<StateView | null>(null);
  const [stats, setStats] = useState<Stats | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [st, sv, ss] = await Promise.all([getStatus(), getState(), getStats()]);
        if (!cancelled) {
          setStatus(st);
          setState(sv);
          setStats(ss);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load status.");
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
  if (status === null || state === null || stats === null) {
    return (
      <main className="page">
        <p role="status">Loading status…</p>
      </main>
    );
  }

  const store = stats.store;
  return (
    <main className="page">
      <h1>Status</h1>
      <p className="banner-warn">LabSyslog is a receive-only lab sink. It never forwards and never relays.</p>
      <dl>
        <div>
          <dt>Ready</dt>
          <dd>
            <strong>{status.ready ? "yes" : "no"}</strong>
          </dd>
        </div>
        <div>
          <dt>Revision</dt>
          <dd>
            <code>{status.revision}</code>
          </dd>
        </div>
        <div>
          <dt>Drifted</dt>
          <dd>
            <strong>{state.drifted ? "yes" : "no"}</strong>
          </dd>
        </div>
        <div>
          <dt>Generation</dt>
          <dd>{state.generation}</dd>
        </div>
      </dl>
      <h2>Listeners</h2>
      <ul>
        <li>
          udp: {status.listeners.udp.bound ? "bound" : "unbound"} <code>{status.listeners.udp.address}</code>
        </li>
        <li>
          tcp: {status.listeners.tcp.bound ? "bound" : "unbound"} <code>{status.listeners.tcp.address}</code>
        </li>
        <li>
          management: {status.listeners.management.bound ? "bound" : "unbound"}{" "}
          <code>{status.listeners.management.address}</code>
        </li>
      </ul>
      <h2>Store caps</h2>
      <dl>
        <div>
          <dt>Messages</dt>
          <dd>
            {store.messages} / {store.maxMessages}
          </dd>
        </div>
        <div>
          <dt>Bytes</dt>
          <dd>
            {store.bytes} / {store.maxBytes}
          </dd>
        </div>
        <div>
          <dt>Policy</dt>
          <dd>{store.fullPolicy}</dd>
        </div>
        <div>
          <dt>Raw retain</dt>
          <dd>{store.rawRetain ? "yes" : "no"}</dd>
        </div>
      </dl>
    </main>
  );
}
