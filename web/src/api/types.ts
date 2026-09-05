export type Problem = {
  type: string;
  title: string;
  status: number;
  detail: string;
  code: string;
};

export type SessionCreated = {
  id: string;
  role: string;
  scopes: string[];
  csrf: string;
  expiresAt: string;
};

export type SessionView = {
  id: string;
  role: string;
  scopes: string[];
  csrf?: string;
  expiresAt?: string;
};

export type SDParam = {
  name: string;
  value: string;
};

export type SDElement = {
  id: string;
  params?: SDParam[];
};

export type Parsed = {
  pri: number;
  facility: number;
  severity: number;
  version: number;
  timestamp?: string;
  hostname?: string;
  appName?: string;
  procID?: string;
  msgID?: string;
  structured?: SDElement[];
  message?: string;
};

export type Message = {
  id: string;
  receivedAt: string;
  transport: string;
  remoteIP?: string;
  remotePort?: number;
  truncated: boolean;
  parseWarning?: string;
  tags?: string[];
  parsed: Parsed;
  raw?: string;
};

export type MessageList = {
  revision: string;
  storeGeneration: number;
  items: Message[];
  nextCursor?: string;
};

export type FilterMatch = {
  sourceCidrs?: string[];
  facilities?: string[];
  severities?: string[];
  severityAtLeast?: string;
  appNames?: string[];
  hostnames?: string[];
  transports?: string[];
};

export type FilterAction = {
  mode: string;
  tag?: string;
};

export type Filter = {
  name: string;
  enabled?: boolean;
  match: FilterMatch;
  action: FilterAction;
};

export type StateView = {
  apiVersion: string;
  kind: string;
  metadata: { name: string };
  spec: {
    filters?: Filter[];
    store?: {
      maxMessages?: number;
      maxBytes?: number;
      fullPolicy?: string;
      rawRetain?: boolean;
    };
    listeners?: {
      udp?: { enabled?: boolean; address?: string };
      tcp?: { enabled?: boolean; address?: string };
      management?: { address?: string };
    };
  };
  revision: string;
  generation: number;
  drifted: boolean;
};

export type ListenerStatus = {
  bound: boolean;
  address: string;
};

export type Status = {
  ready: boolean;
  revision: string;
  drifted: boolean;
  listeners: {
    udp: ListenerStatus;
    tcp: ListenerStatus;
    management: ListenerStatus;
  };
  store: StoreStats;
};

export type StoreStats = {
  messages: number;
  bytes: number;
  generation: number;
  waiters: number;
  evicted: number;
  rejected: number;
  maxMessages: number;
  maxBytes: number;
  fullPolicy: string;
  rawRetain: boolean;
};

export type Stats = {
  store: StoreStats;
};

export type AuditEvent = {
  id: string;
  at: string;
  actor: string;
  operation: string;
  reason?: string;
  revision: string;
};

export type AuditList = {
  items: AuditEvent[];
};

export type Plan = {
  expectedRevision: string;
  nextRevision: string;
  operations: unknown[];
};

export type ApplyResult = {
  revision: string;
  plan: Plan;
};
