import type { LocalShellKind } from '@shared/model/types';
import s from './ui.module.css';

/** Glifo de cada tipo de shell; WSL usa a inicial da distro (como os hosts). */
const GLYPHS: Record<Exclude<LocalShellKind, 'wsl'>, string> = {
  powershell: 'PS',
  pwsh: 'P7',
  cmd: 'C:',
  gitbash: '$_',
  unix: '$_',
};

/** Selo monocromático do shell local, do tamanho de um ícone de menu (FR-176). */
export function ShellIcon({ kind, label }: { kind: LocalShellKind; label: string }) {
  const glyph = kind === 'wsl' ? (Array.from(label)[0] ?? 'L').toUpperCase() : GLYPHS[kind];
  return (
    <span className={s.shellIcon} data-kind={kind} aria-hidden="true">
      {glyph}
    </span>
  );
}
