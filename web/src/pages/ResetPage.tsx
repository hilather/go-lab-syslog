import { FormEvent, useEffect, useState } from "react";
import { APIError, getState, resetState } from "../api/client";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_ADMIN } from "../auth/scopes";
import { canSubmitReset } from "../ui/reset";

export function ResetPage() {
  const { hasScope } = useAuth();
  const allowed = hasScope(SCOPE_ADMIN);
  const [resourceName, setResourceName] = useState("");
  const [phrase, setPhrase] = useState("");
  const [reason, setReason] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const ok = canSubmitReset(phrase, resourceName, confirmed, allowed);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const state = await getState();
        if (!cancelled) {
          setResourceName(state.metadata?.name ?? "");
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load state.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function onSubmit(ev: FormEvent) {
    ev.preventDefault();
    if (!ok) {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await resetState(reason.trim());
      setNotice("Reset completed. Bootstrap YAML was reread and the store was wiped. The file was not written.");
      setPhrase("");
      setConfirmed(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Reset failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page page--narrow">
      <h1>Reset</h1>
      <p>
        Reset rereads bootstrap YAML, wipes the message store and audit ring, and never writes the file. Type
        the resource name {resourceName ? <code>{resourceName}</code> : "(loading)"} to enable the control.
      </p>
      {!allowed ? <p>Requires scope syslog.admin.</p> : null}
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {notice !== "" ? <p role="status">{notice}</p> : null}
      <form className="stack" onSubmit={(e) => void onSubmit(e)}>
        <div className="field">
          <label htmlFor="reset-phrase">Resource name</label>
          <input
            id="reset-phrase"
            value={phrase}
            autoComplete="off"
            spellCheck={false}
            onChange={(e) => setPhrase(e.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="reset-reason">Reason (optional)</label>
          <input id="reset-reason" value={reason} onChange={(e) => setReason(e.target.value)} />
        </div>
        <label>
          <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} /> Wipe
          the store and reread bootstrap
        </label>
        <button type="submit" disabled={!ok || busy}>
          {busy ? "Resetting…" : "Reset LabSyslog"}
        </button>
      </form>
    </main>
  );
}
