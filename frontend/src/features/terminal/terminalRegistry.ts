// Terminais montados por sessão: diálogos devolvem o foco (B3), a busca e a colagem os acessam.
import type { SearchAddon } from '@xterm/addon-search';
import type { Terminal } from '@xterm/xterm';

export interface TerminalHandle {
  term: Terminal;
  search: SearchAddon;
  /** Foca o terminal se a aba estiver visível. */
  focus: () => void;
}

const handles = new Map<string, TerminalHandle>();

export function registerTerminal(sessionId: string, handle: TerminalHandle): () => void {
  handles.set(sessionId, handle);
  return () => {
    if (handles.get(sessionId) === handle) handles.delete(sessionId);
  };
}

export function terminalOf(sessionId: string): TerminalHandle | undefined {
  return handles.get(sessionId);
}

export function focusTerminal(sessionId: string): void {
  handles.get(sessionId)?.focus();
}
