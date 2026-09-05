const POLL_MS = 2500;

export function subscribeLive(onChange: () => void): () => void {
  let poll: ReturnType<typeof setInterval> | undefined;
  const startPoll = () => {
    if (poll !== undefined) {
      return;
    }
    poll = setInterval(onChange, POLL_MS);
  };
  if (typeof EventSource === "undefined") {
    startPoll();
    return () => {
      if (poll !== undefined) clearInterval(poll);
    };
  }
  try {
    const es = new EventSource("/v1/events/stream");
    const handler = () => {
      onChange();
    };
    es.addEventListener("syslog.received", handler);
    es.addEventListener("syslog.deleted", handler);
    es.addEventListener("store.wiped", handler);
    es.onerror = () => {
      es.close();
      startPoll();
    };
    return () => {
      es.close();
      if (poll !== undefined) clearInterval(poll);
    };
  } catch {
    startPoll();
    return () => {
      if (poll !== undefined) clearInterval(poll);
    };
  }
}
