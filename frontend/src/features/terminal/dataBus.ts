// Distribui a saída das sessões para os terminais e guarda o que chega antes da aba montar.
type Sink = (data: string) => void;

const sinks = new Map<string, Sink>();
const backlog = new Map<string, string[]>();
let subscribed = false;

export function subscribeData(): void {
  if (subscribed) return;
  subscribed = true;
  const api = window.openterminal.sessions;
  api.onData(({ sessionId, data }) => {
    const sink = sinks.get(sessionId);
    if (sink) sink(data);
    else backlog.set(sessionId, [...(backlog.get(sessionId) ?? []), data]);
  });
  api.onClosed(({ sessionId }) => {
    forget(sessionId);
  });
}

export function attach(sessionId: string, sink: Sink): () => void {
  sinks.set(sessionId, sink);
  const pending = backlog.get(sessionId);
  if (pending) {
    backlog.delete(sessionId);
    for (const chunk of pending) sink(chunk);
  }
  return () => {
    if (sinks.get(sessionId) === sink) sinks.delete(sessionId);
  };
}

export function forget(sessionId: string): void {
  sinks.delete(sessionId);
  backlog.delete(sessionId);
}
