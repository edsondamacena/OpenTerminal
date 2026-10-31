import type { Appearance, ThemePreference } from '@shared/model/types';
import { accentTokens, DEFAULT_ACCENT } from './accent';
import { THEMES, type ThemeMode, type ThemeTokens } from './tokens';

export function resolveMode(appearance: Appearance, preference: ThemePreference): ThemeMode {
  if (appearance.highContrast) return 'high-contrast';
  const dark = preference === 'system' ? appearance.dark : preference === 'dark';
  if (appearance.glass) return dark ? 'glass-dark' : 'glass-light';
  return dark ? 'solid-dark' : 'solid-light';
}

function cssName(key: string): string {
  return `--${key.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}`;
}

/** Tokens do modo com o destaque escolhido; o alto contraste mantém as cores do sistema. */
export function themeTokens(
  appearance: Appearance,
  preference: ThemePreference,
  accent: string = DEFAULT_ACCENT,
): ThemeTokens {
  const mode = resolveMode(appearance, preference);
  if (mode === 'high-contrast') return THEMES[mode];
  return {
    ...THEMES[mode],
    ...accentTokens(accent, !mode.endsWith('light'), appearance.systemAccent),
  };
}

/** Escreve os tokens como variáveis CSS em :root e retorna os valores aplicados. */
export function applyTheme(
  appearance: Appearance,
  preference: ThemePreference,
  accent: string = DEFAULT_ACCENT,
): ThemeTokens {
  const mode = resolveMode(appearance, preference);
  const tokens = themeTokens(appearance, preference, accent);
  const root = document.documentElement;
  for (const key of Object.keys(tokens) as (keyof ThemeTokens)[]) {
    root.style.setProperty(cssName(key), tokens[key]);
  }
  root.dataset.mode = mode;
  root.style.colorScheme = mode.endsWith('light') ? 'light' : 'dark';
  return tokens;
}
