// Atalhos efetivos (padrão + preferências do usuário) para quem instala o teclado e para as dicas
// da interface: uma dica nunca mostra uma tecla que não funciona (FR-173).
import { useUi } from '../stores/ui';
import {
  buildKeymap,
  comboLabel,
  effectiveShortcuts,
  type Combo,
  type ShortcutAction,
  type ShortcutOverrides,
} from './shortcuts';

function overridesOf(source: Record<string, string> | undefined): ShortcutOverrides {
  return source ?? {};
}

let cached: { source: unknown; keymap: Map<Combo, ShortcutAction> } | null = null;

/** Mapa atual, recalculado só quando as preferências mudam. */
export function currentKeymap(): Map<Combo, ShortcutAction> {
  const source = useUi.getState().settings?.shortcuts;
  if (cached && cached.source === source) return cached.keymap;
  const keymap = buildKeymap(overridesOf(source));
  cached = { source, keymap };
  return keymap;
}

export function useShortcutOverrides(): ShortcutOverrides {
  return overridesOf(useUi((u) => u.settings?.shortcuts));
}

/** Rótulo da primeira combinação da ação, ou `null` se estiver desativada. */
export function useShortcutLabel(action: ShortcutAction): string | null {
  const first = effectiveShortcuts(useShortcutOverrides())[action][0];
  return first ? comboLabel(first) : null;
}

/** "Texto (Ctrl+Shift+B)" quando há atalho; só o texto quando está desativado. */
export function withShortcut(text: string, label: string | null): string {
  return label ? `${text} (${label})` : text;
}
