// Estado de interface do espaço de trabalho que atalhos, paleta e terminais compartilham.
import { create } from 'zustand';
import { isMobile } from '../app/mobile';

export const FONT_MIN = 6;
export const FONT_MAX = 32;

/** O que ocupa a área principal: as abas ou a página de configurações (FR-168). */
export type MainView = 'terminal' | 'settings';

interface PendingPaste {
  sessionId: string;
  text: string;
}

interface WorkspaceState {
  /** Ajuste de fonte por aba (Ctrl+=/−/0), somado ao tamanho das configurações. */
  zoom: Record<string, number>;
  /** Aba com a barra de busca aberta. */
  searchFor: string | null;
  paletteOpen: boolean;
  /** Diálogo de encaminhamentos ativos (barra de status). */
  forwardsOpen: boolean;
  view: MainView;
  pendingPaste: PendingPaste | null;
  /** Aba esperando confirmação para fechar (Ctrl+W com conexão ativa). */
  confirmClose: string | null;
  /** Modo celular: qual tela ocupa o espaço, a lista de hosts ou o console. */
  mobilePane: 'list' | 'terminal';
  setMobilePane: (pane: 'list' | 'terminal') => void;
  zoomBy: (sessionId: string, delta: number, base: number) => void;
  zoomReset: (sessionId: string) => void;
  setSearchFor: (sessionId: string | null) => void;
  setPaletteOpen: (open: boolean) => void;
  setForwardsOpen: (open: boolean) => void;
  setView: (view: MainView) => void;
  setPendingPaste: (paste: PendingPaste | null) => void;
  setConfirmClose: (sessionId: string | null) => void;
}

export const useWorkspace = create<WorkspaceState>((set, get) => ({
  zoom: {},
  searchFor: null,
  paletteOpen: false,
  forwardsOpen: false,
  view: 'terminal',
  pendingPaste: null,
  confirmClose: null,
  mobilePane: 'list',
  setMobilePane(mobilePane) {
    set({ mobilePane });
  },
  zoomBy(sessionId, delta, base) {
    const current = get().zoom[sessionId] ?? defaultZoom();
    const size = Math.min(FONT_MAX, Math.max(FONT_MIN, base + current + delta));
    set({ zoom: { ...get().zoom, [sessionId]: size - base } });
  },
  zoomReset(sessionId) {
    set({
      zoom: Object.fromEntries(Object.entries(get().zoom).filter(([id]) => id !== sessionId)),
    });
  },
  setSearchFor(searchFor) {
    set({ searchFor });
  },
  setPaletteOpen(paletteOpen) {
    set({ paletteOpen });
  },
  setForwardsOpen(forwardsOpen) {
    set({ forwardsOpen });
  },
  setView(view) {
    set({ view });
  },
  setPendingPaste(pendingPaste) {
    set({ pendingPaste });
  },
  setConfirmClose(confirmClose) {
    set({ confirmClose });
  },
}));

// v5: o padrão desceu de novo; o tamanho antigo salvo não vale mais.
const MOBILE_ZOOM_KEY = 'openterminal.mobileZoom.v5';

/** Zoom de aba nova: no celular a fonte do perfil (feita para o PC) começa 7 pontos menor, ou
 *  no último tamanho escolhido com a pinça; no PC, zero. */
export function defaultZoom(): number {
  if (!isMobile()) return 0;
  try {
    const saved = Number(localStorage.getItem(MOBILE_ZOOM_KEY));
    if (localStorage.getItem(MOBILE_ZOOM_KEY) !== null && Number.isFinite(saved)) return saved;
  } catch {
    // Sem armazenamento: fica o padrão.
  }
  return -7;
}

export function rememberMobileZoom(offset: number): void {
  try {
    localStorage.setItem(MOBILE_ZOOM_KEY, String(offset));
  } catch {
    // Sem armazenamento: vale só para esta aba.
  }
}

/** Tamanho de fonte efetivo da aba. */
export function fontSizeOf(sessionId: string, base: number, zoom: Record<string, number>): number {
  return Math.min(FONT_MAX, Math.max(FONT_MIN, base + (zoom[sessionId] ?? defaultZoom())));
}
