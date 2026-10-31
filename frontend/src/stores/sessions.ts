// Abas abertas (FR-012). O estado real de cada sessão vive no processo principal.
import { create } from 'zustand';
import type { LocalShell, SessionKind, SessionState } from '@shared/model/types';
import { forget } from '../features/terminal/dataBus';
import { dropInteractions } from './interactions';

export interface Tab {
  id: string;
  kind: SessionKind;
  title: string;
  hostId: string | null;
  shell: LocalShell | null;
  state: SessionState;
  reason: string | null;
  exitCode: number | null;
  /** RTT do keepalive SSH (FR-172); null antes da primeira medição e em shells locais. */
  latencyMs: number | null;
}

interface SessionsState {
  tabs: Tab[];
  activeId: string | null;
  add: (tab: Omit<Tab, 'reason' | 'exitCode' | 'latencyMs'>) => void;
  setState: (
    id: string,
    state: SessionState,
    reason: string | null,
    exitCode: number | null,
    latencyMs?: number | null,
  ) => void;
  rename: (id: string, title: string) => void;
  /** Move a aba `id` para a posição de `beforeId` (ou para o fim). */
  reorder: (id: string, beforeId: string | null) => void;
  close: (id: string) => void;
  activate: (id: string) => void;
}

export const useSessions = create<SessionsState>((set, get) => ({
  tabs: [],
  activeId: null,
  add(tab) {
    set({
      tabs: [...get().tabs, { ...tab, reason: null, exitCode: null, latencyMs: null }],
      activeId: tab.id,
    });
  },
  setState(id, state, reason, exitCode, latencyMs = null) {
    set({
      tabs: get().tabs.map((t) => (t.id === id ? { ...t, state, reason, exitCode, latencyMs } : t)),
    });
  },
  rename(id, title) {
    const clean = title.trim();
    if (!clean) return;
    set({ tabs: get().tabs.map((t) => (t.id === id ? { ...t, title: clean.slice(0, 100) } : t)) });
  },
  reorder(id, beforeId) {
    const tabs = get().tabs;
    const moving = tabs.find((t) => t.id === id);
    if (!moving || id === beforeId) return;
    const rest = tabs.filter((t) => t.id !== id);
    const index = beforeId === null ? rest.length : rest.findIndex((t) => t.id === beforeId);
    rest.splice(index < 0 ? rest.length : index, 0, moving);
    set({ tabs: rest });
  },
  close(id) {
    const { tabs, activeId } = get();
    const index = tabs.findIndex((t) => t.id === id);
    const next = tabs.filter((t) => t.id !== id);
    const neighbor = next[Math.min(index, next.length - 1)];
    set({ tabs: next, activeId: activeId === id ? (neighbor?.id ?? null) : activeId });
  },
  activate(id) {
    set({ activeId: id });
  },
}));

let subscribed = false;

export function subscribeSessions(): void {
  if (subscribed) return;
  subscribed = true;
  window.openterminal.sessions.onState((e) => {
    useSessions
      .getState()
      .setState(e.sessionId, e.state, e.reason, e.exitCode, e.latencyMs ?? null);
  });
}

export async function closeTab(id: string): Promise<void> {
  useSessions.getState().close(id);
  dropInteractions(id);
  forget(id);
  await window.openterminal.sessions.close(id);
}

export function isLive(state: SessionState): boolean {
  return state !== 'disconnected' && state !== 'error' && state !== 'closed';
}
