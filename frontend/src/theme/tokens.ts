// Tokens visuais do OpenTerminal (FR-031–FR-034). Cores em rgba() ou #rrggbb; a paleta da marca vem
// de brand.gen.ts (gerado de brand/brand.json) e o destaque de accent.ts.
import { accentTokens, DEFAULT_ACCENT } from './accent';
import { BRAND } from './brand.gen';

export type ThemeMode =
  'glass-dark' | 'glass-light' | 'solid-dark' | 'solid-light' | 'high-contrast';

export interface ThemeTokens {
  /** Superfície da moldura; translúcida nos modos de vidro (opacidade mínima por FR-034). */
  surfaceGlass: string;
  /** Superfície elevada (diálogos, menus) sobre a moldura. */
  surfaceRaised: string;
  /** Borda de 1 px com brilho sutil. */
  glassEdge: string;
  /** Painéis opacos: terminal e SFTP (FR-032). */
  panelSolid: string;
  /** Campos de texto e áreas rebaixadas. */
  inputBackground: string;
  textPrimary: string;
  textSecondary: string;
  border: string;
  hover: string;
  selected: string;
  accent: string;
  /** Preenchimento dos botões primários e da marca. */
  accentGradient: string;
  /** Halo de foco e destaque suave em volta do acento. */
  accentSoft: string;
  accentText: string;
  danger: string;
  warning: string;
  success: string;
  focus: string;
  /** Botão fechar da barra de título: vermelho do Windows 11 (FR-133). */
  windowCloseHover: string;
  windowCloseActive: string;
  windowCloseText: string;
  shadow: string;
  /** Brilho de fundo das telas cheias (cofre). */
  backdropGlow: string;
  terminalBackground: string;
  terminalForeground: string;
  terminalCursor: string;
  terminalSelection: string;
  /** Ocorrências da busca no terminal; o xterm exige `#rrggbb`. */
  terminalMatch: string;
  terminalMatchActive: string;
}

const darkTerminal = {
  terminalBackground: BRAND.pretoPiano,
  terminalForeground: '#e8e8e8',
  terminalCursor: '#f5f5f5',
  terminalSelection: 'rgba(255, 255, 255, 0.22)',
  terminalMatch: '#4a3f17',
  terminalMatchActive: '#b8860b',
};

const lightTerminal = {
  terminalBackground: '#fdfdfc',
  terminalForeground: '#141414',
  terminalCursor: '#111111',
  terminalSelection: 'rgba(0, 0, 0, 0.14)',
  terminalMatch: '#fbe7a1',
  terminalMatchActive: '#f2b632',
};

const dark = {
  textPrimary: BRAND.nevoa,
  textSecondary: BRAND.cinza,
  border: 'rgba(255, 255, 255, 0.09)',
  hover: 'rgba(255, 255, 255, 0.06)',
  ...accentTokens(DEFAULT_ACCENT, true, null),
  danger: BRAND.perigo,
  warning: BRAND.aviso,
  success: BRAND.sucesso,
  windowCloseHover: '#c42b1c',
  windowCloseActive: '#b22a1d',
  windowCloseText: '#ffffff',
  shadow: '0 18px 48px rgba(0, 0, 0, 0.70), 0 2px 6px rgba(0, 0, 0, 0.40)',
  // Laca de piano: reflexo branco difuso no alto, sem matiz.
  backdropGlow:
    'radial-gradient(1100px 520px at 18% -18%, rgba(255, 255, 255, 0.06), transparent 62%), linear-gradient(165deg, rgba(255, 255, 255, 0.03) 0%, transparent 38%)',
  panelSolid: '#070707',
  inputBackground: 'rgba(255, 255, 255, 0.04)',
  ...darkTerminal,
};

const light = {
  textPrimary: BRAND.tinta,
  textSecondary: BRAND.chumbo,
  border: 'rgba(0, 0, 0, 0.10)',
  hover: 'rgba(0, 0, 0, 0.05)',
  ...accentTokens(DEFAULT_ACCENT, false, null),
  danger: BRAND.perigoProfundo,
  warning: BRAND.avisoProfundo,
  success: BRAND.sucessoProfundo,
  windowCloseHover: '#c42b1c',
  windowCloseActive: '#b22a1d',
  windowCloseText: '#ffffff',
  shadow: '0 18px 48px rgba(0, 0, 0, 0.14), 0 2px 6px rgba(0, 0, 0, 0.08)',
  backdropGlow:
    'radial-gradient(1100px 520px at 18% -18%, rgba(255, 255, 255, 0.9), transparent 62%), radial-gradient(900px 500px at 110% 110%, rgba(0, 0, 0, 0.05), transparent 60%)',
  panelSolid: '#fdfdfc',
  inputBackground: 'rgba(255, 255, 255, 0.9)',
  ...lightTerminal,
};

export const THEMES: Record<ThemeMode, ThemeTokens> = {
  'glass-dark': {
    ...dark,
    surfaceGlass: 'rgba(6, 6, 6, 0.9)',
    surfaceRaised: 'rgba(22, 22, 22, 0.96)',
    glassEdge:
      'linear-gradient(160deg, rgba(255,255,255,0.24), rgba(255,255,255,0.03) 40%, rgba(255,255,255,0.10))',
  },
  'glass-light': {
    ...light,
    surfaceGlass: 'rgba(246, 246, 244, 0.84)',
    surfaceRaised: 'rgba(255, 255, 255, 0.96)',
    glassEdge:
      'linear-gradient(160deg, rgba(255,255,255,0.95), rgba(255,255,255,0.35) 40%, rgba(0,0,0,0.10))',
  },
  'solid-dark': {
    ...dark,
    surfaceGlass: BRAND.laca,
    surfaceRaised: BRAND.superficie,
    glassEdge: 'linear-gradient(rgba(255,255,255,0.09), rgba(255,255,255,0.09))',
  },
  'solid-light': {
    ...light,
    surfaceGlass: BRAND.papel,
    surfaceRaised: BRAND.branco,
    glassEdge: 'linear-gradient(rgba(0,0,0,0.10), rgba(0,0,0,0.10))',
  },
  // Alto contraste: cores de sistema do Windows, sem translucidez.
  'high-contrast': {
    surfaceGlass: 'Canvas',
    surfaceRaised: 'Canvas',
    glassEdge: 'linear-gradient(CanvasText, CanvasText)',
    panelSolid: 'Canvas',
    inputBackground: 'Canvas',
    textPrimary: 'CanvasText',
    textSecondary: 'CanvasText',
    border: 'CanvasText',
    hover: 'Highlight',
    selected: 'Highlight',
    accent: 'ButtonText',
    accentGradient: 'ButtonText',
    accentSoft: 'Highlight',
    accentText: 'ButtonFace',
    danger: 'CanvasText',
    warning: 'CanvasText',
    success: 'CanvasText',
    focus: 'Highlight',
    windowCloseHover: 'Highlight',
    windowCloseActive: 'Highlight',
    windowCloseText: 'HighlightText',
    shadow: 'none',
    backdropGlow: 'none',
    terminalBackground: 'Canvas',
    terminalForeground: 'CanvasText',
    terminalCursor: 'CanvasText',
    terminalSelection: 'Highlight',
    terminalMatch: '#000080',
    terminalMatchActive: '#008080',
  },
};

export const GLASS_MODES: ThemeMode[] = ['glass-dark', 'glass-light'];
export const SOLID_MODES: ThemeMode[] = ['solid-dark', 'solid-light'];

/** Paleta ANSI própria para o xterm. */
export const ANSI = {
  dark: {
    black: '#1d2026',
    red: '#ff7b72',
    green: '#5cc98b',
    yellow: '#e8c35a',
    blue: '#6aa8f0',
    magenta: '#c792ea',
    cyan: '#56c8d8',
    white: '#cdd2da',
    brightBlack: '#5c6370',
    brightRed: '#ffa198',
    brightGreen: '#8ee0ae',
    brightYellow: '#f5d77f',
    brightBlue: '#9cc6ff',
    brightMagenta: '#dcb2f5',
    brightCyan: '#8ce0ec',
    brightWhite: '#f6f8fa',
  },
  light: {
    black: '#24292f',
    red: '#c4322b',
    green: '#1f7a46',
    yellow: '#8a5d00',
    blue: '#1f6fd1',
    magenta: '#8e44b5',
    cyan: '#0f7c8a',
    white: '#6e7681',
    brightBlack: '#57606a',
    brightRed: '#d9473f',
    brightGreen: '#2a8f55',
    brightYellow: '#a06d00',
    brightBlue: '#3b82e0',
    brightMagenta: '#a35bc8',
    brightCyan: '#16909f',
    brightWhite: '#8c959f',
  },
} as const;
