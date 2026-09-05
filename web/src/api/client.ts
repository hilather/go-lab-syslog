import { assertNoTokenStorage } from "./storage";
import type {
  ApplyResult,
  AuditList,
  Filter,
  Message,
  MessageList,
  Problem,
  SessionCreated,
  SessionView,
  StateView,
  Stats,
  Status,
} from "./types";

export const CSRF_HEADER = "X-LabSyslog-CSRF";

export class APIError extends Error {
  readonly problem: Problem;

  constructor(problem: Problem) {
    super(problem.detail || problem.title || "request failed");
    this.name = "APIError";
    this.problem = problem;
  }
}

let memoryCSRF = "";

export function setMemoryCSRF(value: string): void {
  memoryCSRF = value;
}

export function getMemoryCSRF(): string {
  return memoryCSRF;
}

export function clearMemoryCSRF(): void {
  memoryCSRF = "";
}

function problemFrom(status: number, statusText: string, body: unknown): Problem {
  const fallback: Problem = {
    type: "https://labsyslog.dev/errors/internal_error",
    title: statusText || "error",
    status,
    detail: statusText || "request failed",
    code: status === 401 ? "unauthorized" : status === 403 ? "forbidden" : "internal_error",
  };
  if (!body || typeof body !== "object") {
    return fallback;
  }
  const rec = body as Record<string, unknown>;
  return {
    type: typeof rec.type === "string" ? rec.type : fallback.type,
    title: typeof rec.title === "string" ? rec.title : fallback.title,
    status: typeof rec.status === "number" ? rec.status : fallback.status,
    detail: typeof rec.detail === "string" ? rec.detail : fallback.detail,
    code: typeof rec.code === "string" ? rec.code : fallback.code,
  };
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  assertNoTokenStorage();
  const headers = new Headers(init.headers);
  const method = (init.method ?? "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD" && !headers.has(CSRF_HEADER)) {
    const csrf = getMemoryCSRF();
    if (csrf !== "") {
      headers.set(CSRF_HEADER, csrf);
    }
  }
  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }
  return fetch(path, {
    ...init,
    credentials: "same-origin",
    headers,
  });
}

async function readJSON<T>(resp: Response): Promise<T> {
  const text = await resp.text();
  let parsed: unknown;
  if (text !== "") {
    try {
      parsed = JSON.parse(text) as unknown;
    } catch {
      parsed = undefined;
    }
  }
  if (!resp.ok) {
    throw new APIError(problemFrom(resp.status, resp.statusText, parsed));
  }
  return parsed as T;
}

export async function createSession(authorization: string): Promise<SessionCreated> {
  const resp = await apiFetch("/v1/session", {
    method: "POST",
    headers: { Authorization: authorization },
  });
  const created = await readJSON<SessionCreated>(resp);
  setMemoryCSRF(created.csrf);
  assertNoTokenStorage();
  return created;
}

export function bearerAuthorization(token: string): string {
  return `Bearer ${token}`;
}

export async function getSession(): Promise<SessionView> {
  const view = await readJSON<SessionView>(await apiFetch("/v1/session"));
  if (typeof view.csrf === "string" && view.csrf !== "") {
    setMemoryCSRF(view.csrf);
  }
  return view;
}

export async function deleteSession(): Promise<void> {
  const resp = await apiFetch("/v1/session", { method: "DELETE" });
  if (resp.status === 401 || resp.status === 204) {
    clearMemoryCSRF();
    return;
  }
  await readJSON<unknown>(resp);
  clearMemoryCSRF();
}

export type MessageQuery = {
  facility?: string | undefined;
  severity?: string | undefined;
  appName?: string | undefined;
  hostname?: string | undefined;
  transport?: string | undefined;
  messageContains?: string | undefined;
  limit?: number | undefined;
};

export async function listMessages(q: MessageQuery = {}): Promise<MessageList> {
  const params = new URLSearchParams();
  params.set("limit", String(q.limit ?? 100));
  if (q.facility) params.set("facility", q.facility);
  if (q.severity) params.set("severity", q.severity);
  if (q.appName) params.set("appName", q.appName);
  if (q.hostname) params.set("hostname", q.hostname);
  if (q.transport) params.set("transport", q.transport);
  if (q.messageContains) params.set("messageContains", q.messageContains);
  return readJSON<MessageList>(await apiFetch(`/v1/messages?${params.toString()}`));
}

export async function getMessage(id: string, raw = false): Promise<Message> {
  const suffix = raw ? "?raw=true" : "";
  return readJSON<Message>(await apiFetch(`/v1/messages/${encodeURIComponent(id)}${suffix}`));
}

export async function getMessageRaw(id: string): Promise<string> {
  const resp = await apiFetch(`/v1/messages/${encodeURIComponent(id)}/raw`, {
    headers: { Accept: "application/octet-stream" },
  });
  if (!resp.ok) {
    const text = await resp.text();
    let parsed: unknown;
    try {
      parsed = JSON.parse(text) as unknown;
    } catch {
      parsed = undefined;
    }
    throw new APIError(problemFrom(resp.status, resp.statusText, parsed));
  }
  return resp.text();
}

export async function clearMessages(reason: string): Promise<void> {
  const params = new URLSearchParams();
  if (reason) params.set("reason", reason);
  const q = params.toString();
  const resp = await apiFetch(`/v1/messages:clear${q ? `?${q}` : ""}`, { method: "POST" });
  if (resp.status === 204) {
    return;
  }
  await readJSON<unknown>(resp);
}

export async function getState(): Promise<StateView> {
  return readJSON<StateView>(await apiFetch("/v1/state"));
}

export async function getStatus(): Promise<Status> {
  return readJSON<Status>(await apiFetch("/v1/status"));
}

export async function getStats(): Promise<Stats> {
  return readJSON<Stats>(await apiFetch("/v1/stats"));
}

export async function listAudit(): Promise<AuditList> {
  return readJSON<AuditList>(await apiFetch("/v1/audit"));
}

export async function applyFilters(expectedRevision: string, filters: Filter[], reason: string): Promise<ApplyResult> {
  const idempotencyKey = crypto.randomUUID();
  return readJSON<ApplyResult>(
    await apiFetch("/v1/changes:apply", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify({
        expectedRevision,
        idempotencyKey,
        reason,
        operations: [{ type: "replaceFilters", filters }],
      }),
    }),
  );
}

export async function resetState(reason: string): Promise<StateView> {
  const params = new URLSearchParams();
  if (reason) params.set("reason", reason);
  const q = params.toString();
  return readJSON<StateView>(await apiFetch(`/v1/state:reset${q ? `?${q}` : ""}`, { method: "POST" }));
}
