import { FormEvent, useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { APIError, clearMessages, listMessages, type MessageQuery } from "../api/client";
import type { Message } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE } from "../auth/scopes";
import { FACILITIES, SEVERITIES, facilityName, severityName } from "../ui/keywords";
import { TextBlock } from "../ui/TextBlock";
import { canSubmitClear } from "../ui/reset";
import { subscribeLive } from "../query/live";

export function MessagesPage() {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [query, setQuery] = useState<MessageQuery>({});
  const [draft, setDraft] = useState<MessageQuery>({});
  const [items, setItems] = useState<Message[] | null>(null);
  const [generation, setGeneration] = useState<number | null>(null);
  const [error, setError] = useState("");
  const [phrase, setPhrase] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const ok = canSubmitClear(phrase, confirmed, canWrite);

  const reload = useCallback(async () => {
    const list = await listMessages(query);
    setItems(list.items ?? []);
    setGeneration(list.storeGeneration);
  }, [query]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await reload();
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load messages.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [reload]);

  useEffect(() => {
    return subscribeLive(() => {
      void reload().catch(() => {
        /* keep last good list */
      });
    });
  }, [reload]);

  function onFilter(ev: FormEvent) {
    ev.preventDefault();
    setError("");
    setQuery({ ...draft });
  }

  async function onClear(ev: FormEvent) {
    ev.preventDefault();
    if (!ok) {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await clearMessages("ui: clear messages");
      setNotice("Store cleared.");
      setPhrase("");
      setConfirmed(false);
      await reload();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Clear failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page">
      <h1>Messages</h1>
      <p>
        Live tail of the ephemeral store. LabSyslog never forwards and never relays. Filters are a view of
        stored messages.
      </p>
      {generation !== null ? (
        <p>
          Store generation <span className="chip chip--live">{generation}</span>
        </p>
      ) : null}
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {notice !== "" ? <p role="status">{notice}</p> : null}
      <form className="row" onSubmit={onFilter}>
        <div className="field">
          <label htmlFor="filter-facility">Facility</label>
          <select
            id="filter-facility"
            value={draft.facility ?? ""}
            onChange={(e) => setDraft({ ...draft, facility: e.target.value || undefined })}
          >
            <option value="">any</option>
            {FACILITIES.map((f) => (
              <option key={f} value={f}>
                {f}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="filter-severity">Severity</label>
          <select
            id="filter-severity"
            value={draft.severity ?? ""}
            onChange={(e) => setDraft({ ...draft, severity: e.target.value || undefined })}
          >
            <option value="">any</option>
            {SEVERITIES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="filter-app">App</label>
          <input
            id="filter-app"
            value={draft.appName ?? ""}
            onChange={(e) => setDraft({ ...draft, appName: e.target.value || undefined })}
          />
        </div>
        <div className="field">
          <label htmlFor="filter-host">Host</label>
          <input
            id="filter-host"
            value={draft.hostname ?? ""}
            onChange={(e) => setDraft({ ...draft, hostname: e.target.value || undefined })}
          />
        </div>
        <div className="field">
          <label htmlFor="filter-transport">Transport</label>
          <select
            id="filter-transport"
            value={draft.transport ?? ""}
            onChange={(e) => setDraft({ ...draft, transport: e.target.value || undefined })}
          >
            <option value="">any</option>
            <option value="udp">udp</option>
            <option value="tcp">tcp</option>
          </select>
        </div>
        <div className="field">
          <label htmlFor="filter-contains">Contains</label>
          <input
            id="filter-contains"
            value={draft.messageContains ?? ""}
            onChange={(e) => setDraft({ ...draft, messageContains: e.target.value || undefined })}
          />
        </div>
        <button type="submit">Apply filters</button>
      </form>
      {items === null ? (
        <p role="status">Loading messages…</p>
      ) : items.length === 0 ? (
        <p className="muted">No messages in the current window.</p>
      ) : (
        <table className="data">
          <caption>Newest first. Message bodies are text, never HTML.</caption>
          <thead>
            <tr>
              <th>Received</th>
              <th>Facility</th>
              <th>Severity</th>
              <th>Host</th>
              <th>App</th>
              <th>Transport</th>
              <th>Message</th>
            </tr>
          </thead>
          <tbody>
            {items.map((m) => (
              <tr key={m.id}>
                <td>
                  <Link to={`/messages/${encodeURIComponent(m.id)}`}>{m.receivedAt}</Link>
                </td>
                <td>
                  <span className="chip">{facilityName(m.parsed.facility)}</span>
                </td>
                <td>
                  <span className="chip">{severityName(m.parsed.severity)}</span>
                </td>
                <td>{m.parsed.hostname || "—"}</td>
                <td>{m.parsed.appName || "—"}</td>
                <td>{m.transport}</td>
                <td>
                  <TextBlock className="raw raw--inline" text={m.parsed.message ?? ""} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <section className="stack" aria-labelledby="clear-heading">
        <h2 id="clear-heading">Clear store</h2>
        <p>
          Type <code>messages</code> to enable clear. This wipes the ephemeral window, not bootstrap YAML.
        </p>
        <form className="stack" onSubmit={(e) => void onClear(e)}>
          <div className="field">
            <label htmlFor="clear-phrase">Resource name</label>
            <input
              id="clear-phrase"
              value={phrase}
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => setPhrase(e.target.value)}
            />
          </div>
          <label>
            <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} /> Wipe
            stored messages
          </label>
          <button type="submit" disabled={!ok || busy}>
            {busy ? "Clearing…" : "Clear messages"}
          </button>
        </form>
      </section>
    </main>
  );
}
