// Reconexão automática das abas SSH que caíram (FR-177), com recuo exponencial.
import { create } from 'zustand';
import { useSessions, type Tab } from '../../stores/sessions';
import { useUi } from '../../stores/ui';
import { useVault } from '../../stores/vault';

/** Tentativas antes de desistir e deixar só o botão "Reconectar". */
export const MAX_ATTEMPTS = 6;
const MAX_DELAY_MS = 30_000;

/** Espera antes da tentativa `attempt` (1, 2, 4, 8, 16 e 30 s). */
export function backoffMs(attempt: number): number {
  return Math.min(1_000 * 2 ** Math.max(0, attempt - 1), MAX_DELAY_MS);
}

interface Pending {
  attempt: number;
  /** Instante (Date.now) da próxima tentativa. */
  at: number;
}

interface AutoReconnectState {
  pending: Record<string, Pending>;
}

/** Próxima tentativa por aba, para a faixa mostrar a contagem. */
export const useAutoReconnect = create<AutoReconnectState>(() => ({ pending: {} }));

const timers = new Map<string, ReturnType<typeof setTimeout>>();
/** Abas que já caíram com a conexão estabelecida: só elas entram na reconexão automática. */
const attempts = new Map<string, number>();

function setPending(id: string, value: Pending | null) {
  const rest = Object.fromEntries(
    Object.entries(useAutoReconnect.getState().pending).filter(([key]) => key !== id),
  );
  useAutoReconnect.setState({ pending: value ? { ...rest, [id]: value } : rest });
}

function cancel(id: string) {
  const timer = timers.get(id);
  if (timer) clearTimeout(timer);
  timers.delete(id);
  setPending(id, null);
}

function schedule(tab: Tab) {
  const attempt = (attempts.get(tab.id) ?? 0) + 1;
  if (attempt > MAX_ATTEMPTS) {
    attempts.delete(tab.id);
    setPending(tab.id, null);
    return;
  }
  attempts.set(tab.id, attempt);
  const delay = backoffMs(attempt);
  setPending(tab.id, { attempt, at: Date.now() + delay });
  timers.set(
    tab.id,
    setTimeout(() => {
      timers.delete(tab.id);
      setPending(tab.id, null);
      const current = useSessions.getState().tabs.find((t) => t.id === tab.id);
      // Cofre bloqueado: a reconexão precisa das credenciais; o usuário reconecta depois.
      if (!current || useVault.getState().status?.state !== 'unlocked') return;
      useSessions.getState().setState(tab.id, 'connecting', null, null);
      void window.openterminal.sessions.reconnect(tab.id);
    }, delay),
  );
}

/** Para a contagem da aba (reconectar à mão, fechar a aba). */
export function stopAutoReconnect(id: string): void {
  cancel(id);
  attempts.delete(id);
}

let installed = false;

export function installAutoReconnect(): void {
  if (installed) return;
  installed = true;
  let previous = new Map(useSessions.getState().tabs.map((t) => [t.id, t.state]));
  useSessions.subscribe(({ tabs }) => {
    const enabled = useUi.getState().settings?.autoReconnect ?? false;
    const seen = new Set<string>();
    for (const tab of tabs) {
      seen.add(tab.id);
      const before = previous.get(tab.id);
      if (before === tab.state) continue;
      if (tab.state === 'connected') {
        stopAutoReconnect(tab.id);
        continue;
      }
      if ((tab.kind !== 'ssh' && tab.kind !== 'telnet') || !enabled || timers.has(tab.id)) continue;
      // Queda de uma sessão que estava no ar, ou nova falha durante as tentativas.
      const dropped = tab.state === 'disconnected' && before === 'connected';
      const retryFailed =
        (tab.state === 'disconnected' || tab.state === 'error') && attempts.has(tab.id);
      if (dropped || retryFailed) schedule(tab);
    }
    for (const id of previous.keys()) if (!seen.has(id)) stopAutoReconnect(id);
    previous = new Map(tabs.map((t) => [t.id, t.state]));
  });
}
