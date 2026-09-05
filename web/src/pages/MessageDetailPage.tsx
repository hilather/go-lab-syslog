import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { APIError, getMessage, getMessageRaw } from "../api/client";
import type { Message } from "../api/types";
import { facilityName, severityName } from "../ui/keywords";
import { TextBlock } from "../ui/TextBlock";

export function MessageDetailPage() {
  const { id } = useParams();
  const [msg, setMsg] = useState<Message | null>(null);
  const [raw, setRaw] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!id) {
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const [m, rawText] = await Promise.all([
          getMessage(id, true),
          getMessageRaw(id).catch(() => ""),
        ]);
        if (!cancelled) {
          setMsg(m);
          setRaw(rawText);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load message.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (error !== "") {
    return (
      <main className="page">
        <p className="banner-error" role="alert">
          {error}
        </p>
        <p>
          <Link to="/">Back to messages</Link>
        </p>
      </main>
    );
  }
  if (msg === null) {
    return (
      <main className="page">
        <p role="status">Loading message…</p>
      </main>
    );
  }

  const p = msg.parsed;
  return (
    <main className="page">
      <p>
        <Link to="/">Back to messages</Link>
      </p>
      <h1>Message {msg.id}</h1>
      <dl>
        <div>
          <dt>Received</dt>
          <dd>{msg.receivedAt}</dd>
        </div>
        <div>
          <dt>Transport</dt>
          <dd>{msg.transport}</dd>
        </div>
        <div>
          <dt>Remote</dt>
          <dd>
            {msg.remoteIP || "—"}
            {msg.remotePort ? `:${msg.remotePort}` : ""}
          </dd>
        </div>
        <div>
          <dt>PRI</dt>
          <dd>{p.pri}</dd>
        </div>
        <div>
          <dt>Facility</dt>
          <dd>
            <span className="chip">{facilityName(p.facility)}</span>
          </dd>
        </div>
        <div>
          <dt>Severity</dt>
          <dd>
            <span className="chip">{severityName(p.severity)}</span>
          </dd>
        </div>
        <div>
          <dt>Version</dt>
          <dd>{p.version === 1 ? "rfc5424" : "rfc3164"}</dd>
        </div>
        <div>
          <dt>Hostname</dt>
          <dd>{p.hostname || "—"}</dd>
        </div>
        <div>
          <dt>App</dt>
          <dd>{p.appName || "—"}</dd>
        </div>
        <div>
          <dt>ProcID</dt>
          <dd>{p.procID || "—"}</dd>
        </div>
        <div>
          <dt>MsgID</dt>
          <dd>{p.msgID || "—"}</dd>
        </div>
        {msg.parseWarning ? (
          <div>
            <dt>Parse warning</dt>
            <dd>{msg.parseWarning}</dd>
          </div>
        ) : null}
        {msg.tags && msg.tags.length > 0 ? (
          <div>
            <dt>Tags</dt>
            <dd>{msg.tags.join(", ")}</dd>
          </div>
        ) : null}
      </dl>
      <h2>Message</h2>
      <TextBlock className="raw" text={p.message ?? ""} />
      {(p.structured ?? []).length > 0 ? (
        <>
          <h2>Structured data</h2>
          <ul>
            {(p.structured ?? []).map((sd) => (
              <li key={sd.id}>
                <code>{sd.id}</code>
                <ul>
                  {(sd.params ?? []).map((param) => (
                    <li key={`${sd.id}-${param.name}`}>
                      {param.name}={param.value}
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ul>
        </>
      ) : null}
      <h2>Raw</h2>
      <TextBlock className="raw" text={raw} />
    </main>
  );
}
