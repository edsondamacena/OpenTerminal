// Ações do espaço de trabalho usadas pelos atalhos (FR-140) e pela paleta (FR-141).
import type { ShortcutAction } from '../../app/shortcuts';
import { closeTab, isLive, useSessions, type Tab } from '../../stores/sessions';
import { useShells } from '../../stores/shells';
import { useUi } from '../../stores/ui';
import { useWorkspace } from '../../stores/workspace';
import { toastError } from '../../ui/Toast';
import { openLocal } from './openLocal';

function activeTab() {
  const { tabs, activeId } = useSessions.getState();
  return tabs.find((tab) => tab.id === activeId) ?? null;
}

/** Aba ativa com terminal (busca e zoom não valem para SFTP). */
export function activeTerminalId(): string | null {
  const tab = activeTab();
  return tab && tab.kind !== 'sftp' ? tab.id : null;
}

export function openSearch(): void {
  const id = activeTerminalId();
  if (!id) return;
  const workspace = useWorkspace.getState();
  if (workspace.searchFor === id) {
    // Já aberta: só volta o foco para o campo.
    document.querySelector<HTMLInputElement>(`[data-search-for="${id}"] input`)?.select();
    return;
  }
  workspace.setSearchFor(id);
}

/** Fecha a aba ativa; sessão viva (SSH, SFTP ou shell local) sempre pede confirmação antes. */
export function closeActiveTab(): void {
  const tab = activeTab();
  if (tab) requestCloseTab(tab);
}

export function requestCloseTab(tab: Tab): void {
  if (isLive(tab.state)) useWorkspace.getState().setConfirmClose(tab.id);
  else void closeTab(tab.id);
}

export function cycleTab(step: 1 | -1): void {
  const { tabs, activeId, activate } = useSessions.getState();
  if (tabs.length < 2) return;
  const index = tabs.findIndex((tab) => tab.id === activeId);
  const next = tabs[(index + step + tabs.length) % tabs.length];
  if (next) activate(next.id);
}

function zoom(delta: number | null): void {
  const id = activeTerminalId();
  const base = useUi.getState().settings?.terminalFontSize ?? 14;
  if (!id) return;
  if (delta === null) useWorkspace.getState().zoomReset(id);
  else useWorkspace.getState().zoomBy(id, delta, base);
}

export async function toggleTheme(): Promise<void> {
  const res = await window.openterminal.app.updateUiSettings({
    theme: useUi.getState().dark ? 'light' : 'dark',
  });
  if (res.ok) useUi.getState().setSettings(res.value);
  else toastError(res.error);
}

/** Recolhe ou expande a barra lateral (FR-167); a interface muda antes de salvar. */
export function toggleSidebar(): void {
  const ui = useUi.getState();
  if (!ui.settings) return;
  const sidebarCollapsed = !ui.settings.sidebarCollapsed;
  ui.setSettings({ ...ui.settings, sidebarCollapsed });
  void window.openterminal.app.updateUiSettings({ sidebarCollapsed }).then((res) => {
    if (!res.ok) toastError(res.error);
  });
}

/** Algum diálogo (que não a paleta) está aberto: atalhos ficam parados. */
function dialogOpen(): boolean {
  return document.querySelector('[role="dialog"][aria-modal="true"]:not([data-palette])') !== null;
}

export function runShortcut(action: ShortcutAction, repeat: boolean): void {
  const workspace = useWorkspace.getState();
  if (action === 'palette') {
    if (!repeat && !dialogOpen()) workspace.setPaletteOpen(!workspace.paletteOpen);
    return;
  }
  if (dialogOpen() || workspace.paletteOpen) return;
  switch (action) {
    case 'newTab':
      if (!repeat) void openLocal(useUi.getState().settings?.defaultShell ?? 'powershell');
      break;
    case 'newPowerShell':
      // Sem PowerShell (Linux), o atalho abre o shell padrão.
      if (!repeat)
        void openLocal(
          useShells.getState().shells.some((s) => s.id === 'powershell')
            ? 'powershell'
            : (useUi.getState().settings?.defaultShell ?? 'powershell'),
        );
      break;
    case 'closeTab':
      if (!repeat) closeActiveTab();
      break;
    case 'nextTab':
      cycleTab(1);
      break;
    case 'prevTab':
      cycleTab(-1);
      break;
    case 'search':
      openSearch();
      break;
    case 'toggleSidebar':
      if (!repeat) toggleSidebar();
      break;
    case 'zoomIn':
      zoom(1);
      break;
    case 'zoomOut':
      zoom(-1);
      break;
    case 'zoomReset':
      zoom(null);
      break;
  }
}
