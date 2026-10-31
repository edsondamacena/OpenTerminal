// Atalhos globais (FR-140, FR-173). Capturados na fase de captura da janela, antes do xterm, para
// não virarem caracteres de controle no shell; o que não é atalho segue para o terminal.
// Padrão em Ctrl+Shift: Ctrl+K, Ctrl+T e Ctrl+B pertencem ao readline e ao tmux. Ctrl+W é exceção:
// no Chromium ele fecha a janela inteira, então o app o toma para fechar a aba (com confirmação).

export type ShortcutAction =
  | 'newTab'
  | 'newPowerShell'
  | 'closeTab'
  | 'nextTab'
  | 'prevTab'
  | 'search'
  | 'palette'
  | 'zoomIn'
  | 'zoomOut'
  | 'zoomReset'
  | 'toggleSidebar';

export interface KeyLike {
  code: string;
  ctrlKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  metaKey: boolean;
}

/** Combinação como texto: modificadores na ordem Ctrl+Alt+Shift e o `code` físico sem `Key`. */
export type Combo = string;

/** Mapa gravado nas preferências: só as ações alteradas; `""` = desativado. */
export type ShortcutOverrides = Partial<Record<ShortcutAction, Combo>>;

export const ACTIONS: readonly ShortcutAction[] = [
  'newTab',
  'newPowerShell',
  'closeTab',
  'nextTab',
  'prevTab',
  'palette',
  'search',
  'toggleSidebar',
  'zoomIn',
  'zoomOut',
  'zoomReset',
];

/** Padrões; o primeiro de cada lista é o que aparece nas dicas. */
export const DEFAULT_SHORTCUTS: Record<ShortcutAction, readonly Combo[]> = {
  newTab: ['Ctrl+Shift+T'],
  newPowerShell: [],
  closeTab: ['Ctrl+W', 'Ctrl+Shift+W'],
  nextTab: ['Ctrl+Tab'],
  prevTab: ['Ctrl+Shift+Tab'],
  palette: ['Ctrl+Shift+P'],
  search: ['Ctrl+Shift+F'],
  toggleSidebar: ['Ctrl+Shift+B'],
  // Ctrl+Shift+= para teclados em que o + exige Shift.
  zoomIn: ['Ctrl+Equal', 'Ctrl+NumpadAdd', 'Ctrl+Shift+Equal'],
  zoomOut: ['Ctrl+Minus', 'Ctrl+NumpadSubtract'],
  zoomReset: ['Ctrl+Digit0', 'Ctrl+Numpad0'],
};

const MODIFIER_CODES = new Set([
  'ControlLeft',
  'ControlRight',
  'ShiftLeft',
  'ShiftRight',
  'AltLeft',
  'AltRight',
  'MetaLeft',
  'MetaRight',
]);

/** Combinação de um evento, ou null para tecla de modificador sozinha. */
export function comboOf(e: KeyLike): Combo | null {
  if (MODIFIER_CODES.has(e.code) || e.metaKey) return null;
  const key = e.code.startsWith('Key') ? e.code.slice(3) : e.code;
  return [e.ctrlKey && 'Ctrl', e.altKey && 'Alt', e.shiftKey && 'Shift', key]
    .filter(Boolean)
    .join('+');
}

/** Combinações efetivas de cada ação: o valor do usuário substitui todos os padrões. */
export function effectiveShortcuts(
  overrides: ShortcutOverrides = {},
): Record<ShortcutAction, readonly Combo[]> {
  const out = { ...DEFAULT_SHORTCUTS };
  for (const action of ACTIONS) {
    const custom = overrides[action];
    if (custom !== undefined) out[action] = custom === '' ? [] : [custom];
  }
  return out;
}

export function buildKeymap(overrides: ShortcutOverrides = {}): Map<Combo, ShortcutAction> {
  const map = new Map<Combo, ShortcutAction>();
  const effective = effectiveShortcuts(overrides);
  for (const action of ACTIONS) {
    for (const combo of effective[action]) if (!map.has(combo)) map.set(combo, action);
  }
  return map;
}

export function matchShortcut(e: KeyLike, keymap = buildKeymap()): ShortcutAction | null {
  const combo = comboOf(e);
  return combo ? (keymap.get(combo) ?? null) : null;
}

/** Ação que já usa a combinação (para recusar conflito), exceto a própria. */
export function conflictOf(
  combo: Combo,
  action: ShortcutAction,
  overrides: ShortcutOverrides,
): ShortcutAction | null {
  const effective = effectiveShortcuts(overrides);
  return ACTIONS.find((a) => a !== action && effective[a].includes(combo)) ?? null;
}

/** Combinação que tira uma tecla do terminal: Ctrl ou Ctrl+Alt (AltGr) sem Shift. */
export function stealsFromTerminal(combo: Combo): boolean {
  const parts = combo.split('+');
  const key = parts[parts.length - 1] ?? '';
  return parts.includes('Ctrl') && !parts.includes('Shift') && /^[A-Z]$/.test(key);
}

const KEY_LABELS: Record<string, string> = {
  Equal: '=',
  Minus: '−',
  NumpadAdd: 'Num +',
  NumpadSubtract: 'Num −',
  Comma: ',',
  Period: '.',
  Slash: '/',
  Backquote: '`',
  BracketLeft: '[',
  BracketRight: ']',
  Semicolon: ';',
  Quote: "'",
  Backslash: '\\',
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
};

/** Texto para a interface: `Ctrl+Digit0` → `Ctrl+0`. */
export function comboLabel(combo: Combo): string {
  return combo
    .split('+')
    .map((part) =>
      part.startsWith('Digit')
        ? part.slice(5)
        : part.startsWith('Numpad') && /\d$/.test(part)
          ? `Num ${part.slice(6)}`
          : (KEY_LABELS[part] ?? part),
    )
    .join('+');
}

const BROWSER_KEYS = new Set(['KeyT', 'KeyN', 'KeyW', 'KeyQ']);

/** Instala os atalhos na janela; `keymap` é lido a cada tecla (muda nas configurações). */
export function installShortcuts(
  run: (action: ShortcutAction, repeat: boolean) => void,
  keymap: () => Map<Combo, ShortcutAction> = () => buildKeymap(),
) {
  const onKeyDown = (e: KeyboardEvent) => {
    // Gravando um atalho nas configurações: a tecla é para o gravador.
    if (e.target instanceof Element && e.target.closest('[data-shortcut-recorder]')) return;
    const action = matchShortcut(e, keymap());
    if (!action) return;
    e.preventDefault();
    e.stopPropagation();
    run(action, e.repeat);
  };
  // Depois do xterm (fase de bolha): o que sobrou de Ctrl+T/N/W não pode virar aba ou janela
  // nova do Chromium nem fechar o app. No terminal o xterm já mandou o caractere ao shell.
  const onBrowserKey = (e: KeyboardEvent) => {
    if (e.ctrlKey && !e.altKey && BROWSER_KEYS.has(e.code)) e.preventDefault();
    // Ctrl+Shift+C é copiar no app; no Chromium seria o inspetor do DevTools.
    if (e.ctrlKey && e.shiftKey && !e.altKey && e.code === 'KeyC') e.preventDefault();
  };
  window.addEventListener('keydown', onKeyDown, true);
  window.addEventListener('keydown', onBrowserKey);
  return () => {
    window.removeEventListener('keydown', onKeyDown, true);
    window.removeEventListener('keydown', onBrowserKey);
  };
}
