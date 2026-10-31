import { create } from 'zustand';
import type { Appearance, Settings } from '@shared/model/types';
import { applyTheme, resolveMode } from '../theme/apply';
import { THEMES, type ThemeTokens } from '../theme/tokens';
import { useShells } from './shells';

interface UiState {
  appearance: Appearance;
  settings: Settings | null;
  tokens: ThemeTokens;
  /** Tema efetivo escuro (preferência do usuário ou do Windows). */
  dark: boolean;
  setAppearance: (a: Appearance) => void;
  setSettings: (s: Settings) => void;
}

const initialAppearance: Appearance = {
  glass: false,
  highContrast: false,
  dark: true,
  systemAccent: null,
};

function derive(appearance: Appearance, settings: Settings | null) {
  const preference = settings?.theme ?? 'system';
  return {
    tokens: applyTheme(appearance, preference, settings?.accent),
    dark: !resolveMode(appearance, preference).endsWith('light'),
  };
}

export const useUi = create<UiState>((set, get) => ({
  appearance: initialAppearance,
  settings: null,
  tokens: THEMES['solid-dark'],
  dark: true,
  setAppearance(appearance) {
    set({ appearance, ...derive(appearance, get().settings) });
  },
  setSettings(settings) {
    set({ settings, ...derive(get().appearance, settings) });
  },
}));

export async function loadUi(): Promise<void> {
  const api = window.openterminal.app;
  const [appearance, settings] = await Promise.all([
    api.getAppearance(),
    api.getUiSettings(),
    useShells.getState().load(),
  ]);
  if (settings.ok) useUi.getState().setSettings(settings.value);
  if (appearance.ok) useUi.getState().setAppearance(appearance.value);
  api.onAppearanceChanged((a) => {
    useUi.getState().setAppearance(a);
  });
  // Só no build e2e (TestService ligado): o teste de aparência força o modo que viria do Windows.
  if ((window as { go?: { testsvc?: unknown } }).go?.testsvc !== undefined) {
    (window as { __setAppearance?: (a: Partial<Appearance>) => void }).__setAppearance = (a) => {
      useUi.getState().setAppearance({ ...useUi.getState().appearance, ...a });
    };
  }
}
