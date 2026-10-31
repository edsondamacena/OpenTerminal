// Perfil de terminal efetivo (FR-156, FR-157): override do host ou global, aplicado ao xterm
// sem recriar o terminal. Funções puras para a amostra das configurações e para as abas.
import type { ITheme, Terminal } from '@xterm/xterm';
import type { ColorScheme, TerminalProfile } from '@shared/model/types';

export const BRAND_SCHEME_ID = '00000000-0000-4000-8000-000000000001';

/** Esquema da marca, usado até a lista do processo principal chegar. */
export const BRAND_SCHEME: ColorScheme = {
  id: BRAND_SCHEME_ID,
  name: 'OpenTerminal Dark',
  origin: 'builtin',
  foreground: '#e8e8e8',
  background: '#050505',
  cursor: '#f5f5f5',
  selection: '#ffffff38',
  ansi: [
    '#1d2026',
    '#ff7b72',
    '#5cc98b',
    '#e8c35a',
    '#6aa8f0',
    '#c792ea',
    '#56c8d8',
    '#cdd2da',
    '#5c6370',
    '#ffa198',
    '#8ee0ae',
    '#f5d77f',
    '#9cc6ff',
    '#dcb2f5',
    '#8ce0ec',
    '#f6f8fa',
  ],
};

/** Padrões do data-model, usados até o perfil global chegar. */
export const DEFAULT_PROFILE: TerminalProfile = {
  id: '00000000-0000-4000-8000-000000000002',
  name: 'Padrão',
  isGlobal: true,
  fontFamily: 'JetBrains Mono',
  fontSize: 14,
  fontWeight: 400,
  fontWeightBold: 700,
  lineHeight: 1.2,
  letterSpacing: 0,
  ligatures: false,
  cursorStyle: 'block',
  cursorBlink: true,
  cursorColor: null,
  scrollback: 10_000,
  backgroundOpacity: 1,
  blur: 0,
  padding: 12,
  colorSchemeId: BRAND_SCHEME_ID,
};

const ANSI_KEYS = [
  'black',
  'red',
  'green',
  'yellow',
  'blue',
  'magenta',
  'cyan',
  'white',
  'brightBlack',
  'brightRed',
  'brightGreen',
  'brightYellow',
  'brightBlue',
  'brightMagenta',
  'brightCyan',
  'brightWhite',
] as const;

/** `#rrggbb` + opacidade 0–1 → `rgba(...)`. */
export function withAlpha(hex: string, alpha: number): string {
  const n = parseInt(hex.slice(1, 7), 16);
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  return `rgba(${String(r)}, ${String(g)}, ${String(b)}, ${String(alpha)})`;
}

export function schemeTheme(scheme: ColorScheme, profile: TerminalProfile): ITheme {
  const theme: ITheme = {
    background:
      profile.backgroundOpacity < 1
        ? withAlpha(scheme.background, profile.backgroundOpacity)
        : scheme.background,
    foreground: scheme.foreground,
    cursor: profile.cursorColor ?? scheme.cursor,
    cursorAccent: scheme.background,
    selectionBackground: scheme.selection,
  };
  ANSI_KEYS.forEach((key, i) => {
    const color = scheme.ansi[i];
    if (color) theme[key] = color;
  });
  return theme;
}

/** Família CSS com reservas monoespaçadas, para glifos que a fonte escolhida não tem. */
export function cssFontFamily(family: string): string {
  const quoted = /^[\w-]+$/.test(family) ? family : `"${family.replace(/"/g, '')}"`;
  return `${quoted}, "Cascadia Mono", Consolas, monospace`;
}

export function schemeOf(schemes: ColorScheme[], id: string): ColorScheme {
  return schemes.find((x) => x.id === id) ?? schemes[0] ?? BRAND_SCHEME;
}

/**
 * Aplica o perfil a um terminal aberto (sem recriar: o conteúdo e a sessão ficam). `fontSize`
 * já inclui o zoom da aba. Espera a fonte carregar para o xterm medir a célula certa.
 */
export async function applyProfile(
  term: Terminal,
  profile: TerminalProfile,
  scheme: ColorScheme,
  fontSize: number,
): Promise<void> {
  const family = cssFontFamily(profile.fontFamily);
  try {
    await Promise.all([
      document.fonts.load(`${String(profile.fontWeight)} ${String(fontSize)}px ${family}`),
      document.fonts.load(`${String(profile.fontWeightBold)} ${String(fontSize)}px ${family}`),
    ]);
  } catch {
    // Fonte indisponível: o xterm usa as reservas.
  }
  const options = term.options;
  // O xterm pode ter medido a célula com a fonte de reserva, antes de a fonte carregar; a mesma
  // família de novo não remede. Trocar e voltar força a medida com a fonte já carregada.
  if (options.fontFamily === family) options.fontFamily = 'monospace';
  options.fontFamily = family;
  options.fontSize = fontSize;
  options.fontWeight = profile.fontWeight;
  options.fontWeightBold = profile.fontWeightBold;
  options.lineHeight = profile.lineHeight;
  options.letterSpacing = profile.letterSpacing;
  options.cursorStyle = profile.cursorStyle;
  options.cursorBlink = profile.cursorBlink;
  options.scrollback = profile.scrollback;
  // Transparência custa desempenho no renderer: só com opacidade abaixo de 100%.
  options.allowTransparency = profile.backgroundOpacity < 1;
  options.theme = schemeTheme(scheme, profile);
}
