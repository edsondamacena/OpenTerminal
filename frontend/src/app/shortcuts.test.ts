import { describe, expect, it, vi } from 'vitest';
import {
  buildKeymap,
  comboLabel,
  comboOf,
  conflictOf,
  installShortcuts,
  matchShortcut,
  stealsFromTerminal,
  type KeyLike,
} from './shortcuts';

const key = (code: string, mods: Partial<KeyLike> = {}): KeyLike => ({
  code,
  ctrlKey: true,
  shiftKey: false,
  altKey: false,
  metaKey: false,
  ...mods,
});
const shift = { shiftKey: true };

describe('matchShortcut (FR-173)', () => {
  it.each([
    [key('KeyT', shift), 'newTab'],
    [key('KeyW', shift), 'closeTab'],
    [key('KeyW'), 'closeTab'],
    [key('Tab'), 'nextTab'],
    [key('Tab', shift), 'prevTab'],
    [key('KeyF', shift), 'search'],
    [key('KeyP', shift), 'palette'],
    [key('KeyB', shift), 'toggleSidebar'],
    [key('Equal'), 'zoomIn'],
    [key('Equal', shift), 'zoomIn'],
    [key('NumpadAdd'), 'zoomIn'],
    [key('Minus'), 'zoomOut'],
    [key('NumpadSubtract'), 'zoomOut'],
    [key('Digit0'), 'zoomReset'],
    [key('Numpad0'), 'zoomReset'],
  ] as const)('%o → %s', (event, action) => {
    expect(matchShortcut(event)).toBe(action);
  });

  it('deixa as teclas do readline e do tmux para o terminal', () => {
    for (const code of ['KeyK', 'KeyT', 'KeyB', 'KeyC', 'KeyF']) {
      expect(matchShortcut(key(code))).toBeNull();
    }
    expect(matchShortcut(key('KeyT', { ctrlKey: false, shiftKey: true }))).toBeNull();
    expect(matchShortcut(key('KeyT', { altKey: true, shiftKey: true }))).toBeNull(); // AltGr
    expect(matchShortcut(key('KeyP', { metaKey: true, shiftKey: true }))).toBeNull();
  });

  it('usa o mapa do usuário: remapear e desativar', () => {
    const keymap = buildKeymap({ closeTab: 'Ctrl+Alt+W', palette: '' });
    expect(matchShortcut(key('KeyW', { altKey: true }), keymap)).toBe('closeTab');
    expect(matchShortcut(key('KeyW', shift), keymap)).toBeNull();
    expect(matchShortcut(key('KeyP', shift), keymap)).toBeNull();
    expect(matchShortcut(key('KeyT', shift), keymap)).toBe('newTab');
  });
});

describe('gravação de atalhos', () => {
  it('monta a combinação na ordem Ctrl+Alt+Shift', () => {
    expect(comboOf(key('KeyT', { shiftKey: true, altKey: true }))).toBe('Ctrl+Alt+Shift+T');
    expect(comboOf(key('F5', { ctrlKey: false }))).toBe('F5');
    expect(comboOf(key('ShiftLeft', shift))).toBeNull();
  });

  it('acha conflito com outra ação', () => {
    expect(conflictOf('Ctrl+Shift+W', 'newTab', {})).toBe('closeTab');
    expect(conflictOf('Ctrl+Shift+W', 'closeTab', {})).toBeNull();
    expect(conflictOf('Ctrl+Shift+W', 'newTab', { closeTab: '' })).toBeNull();
  });

  it('avisa quando a combinação tira uma tecla do terminal', () => {
    expect(stealsFromTerminal('Ctrl+W')).toBe(true);
    expect(stealsFromTerminal('Ctrl+Alt+W')).toBe(true);
    expect(stealsFromTerminal('Ctrl+Shift+W')).toBe(false);
    expect(stealsFromTerminal('Ctrl+Tab')).toBe(false);
  });

  it('mostra rótulos legíveis', () => {
    expect(comboLabel('Ctrl+Digit0')).toBe('Ctrl+0');
    expect(comboLabel('Ctrl+Equal')).toBe('Ctrl+=');
    expect(comboLabel('Ctrl+Shift+T')).toBe('Ctrl+Shift+T');
  });
});

describe('teclas do navegador', () => {
  // Chromium do Linux: o que o app não usar não pode abrir aba, janela nem o DevTools.
  const pressiona = (code: string, mods: { ctrlKey?: boolean; shiftKey?: boolean }) => {
    const ouvintes: ((e: KeyboardEvent) => void)[] = [];
    vi.stubGlobal('Element', class {});
    vi.stubGlobal('window', { addEventListener: (_: string, f: (e: KeyboardEvent) => void) => ouvintes.push(f), removeEventListener: () => {} });
    const desliga = installShortcuts(() => {}, () => new Map());
    const e = { code, ctrlKey: false, shiftKey: false, altKey: false, metaKey: false, repeat: false, target: null,
      preventDefault: vi.fn(), stopPropagation: vi.fn(), ...mods } as unknown as KeyboardEvent;
    ouvintes.forEach((f) => f(e));
    desliga();
    vi.unstubAllGlobals();
    return (e.preventDefault as ReturnType<typeof vi.fn>).mock.calls.length > 0;
  };

  it('Ctrl+Shift+C não abre o inspetor do DevTools', () => {
    expect(pressiona('KeyC', { ctrlKey: true, shiftKey: true })).toBe(true);
  });

  it('Ctrl+C continua livre para o terminal (interromper)', () => {
    expect(pressiona('KeyC', { ctrlKey: true })).toBe(false);
  });
});
