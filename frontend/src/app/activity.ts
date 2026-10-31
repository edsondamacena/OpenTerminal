// Reporta atividade do usuário para o bloqueio automático (FR-003).
// Throttle de 30 s com envio também na borda final: a última interação sempre conta.
const INTERVAL_MS = 30_000;

export function trackActivity(report: () => void, interval = INTERVAL_MS): () => void {
  let last = 0;
  let trailing: ReturnType<typeof setTimeout> | null = null;

  const onActivity = () => {
    const now = Date.now();
    if (now - last >= interval) {
      last = now;
      report();
      return;
    }
    trailing ??= setTimeout(
      () => {
        trailing = null;
        last = Date.now();
        report();
      },
      interval - (now - last),
    );
  };

  const events = ['keydown', 'pointerdown', 'wheel'] as const;
  // Captura: inclui eventos do terminal, que o xterm consome.
  for (const e of events) window.addEventListener(e, onActivity, { capture: true, passive: true });
  return () => {
    if (trailing) clearTimeout(trailing);
    for (const e of events) window.removeEventListener(e, onActivity, { capture: true });
  };
}
