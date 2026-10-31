// Shells locais detectados pelo Go (FR-176): PowerShell, PowerShell 7, cmd, Git Bash e distros WSL.
import { create } from 'zustand';
import type { LocalShell, LocalShellInfo } from '@shared/model/types';
import { t } from '../i18n/pt-BR';

/** Nomes curtos nas abas e menus; distros WSL ficam como "Ubuntu (WSL)". */
const LABELS: Partial<Record<LocalShellInfo['kind'], string>> = {
  powershell: t.terminal.powershell,
  pwsh: t.terminal.pwsh,
  cmd: t.terminal.cmd,
  gitbash: t.terminal.gitbash,
};

function labelOf(shell: LocalShellInfo): string {
  return shell.kind === 'wsl' ? t.terminal.wsl(shell.label) : (LABELS[shell.kind] ?? shell.label);
}

/** Até a detecção responder (ou se falhar), os dois shells que todo Windows tem. */
const FALLBACK: LocalShellInfo[] = [
  { id: 'powershell', label: t.terminal.powershell, kind: 'powershell' },
  { id: 'cmd', label: t.terminal.cmd, kind: 'cmd' },
];

interface ShellsState {
  shells: LocalShellInfo[];
  load: () => Promise<void>;
}

export const useShells = create<ShellsState>((set) => ({
  shells: FALLBACK,
  async load() {
    const res = await window.openterminal.app.listLocalShells();
    if (res.ok && res.value.length > 0)
      set({ shells: res.value.map((s) => ({ ...s, label: labelOf(s) })) });
  },
}));

export function shellInfo(id: LocalShell): LocalShellInfo {
  return (
    useShells.getState().shells.find((s) => s.id === id) ?? {
      id,
      label: id.startsWith('wsl:') ? t.terminal.wsl(id.slice(4)) : id,
      kind: id.startsWith('wsl:') ? 'wsl' : 'powershell',
    }
  );
}
