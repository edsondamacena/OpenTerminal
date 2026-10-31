// Perfil global, overrides por host, esquemas e fontes (US4). Mudanças do usuário entram aqui
// antes de chegar ao processo principal, para as abas e a amostra reagirem na hora (FR-151).
import { create } from 'zustand';
import type { ColorScheme, FontInfo, TerminalProfile } from '@shared/model/types';
import { BRAND_SCHEME, DEFAULT_PROFILE } from '../features/terminal/profile';
import { toastError } from '../ui/Toast';

interface ProfilesState {
  global: TerminalProfile;
  /** Override por host; ausente = ainda não consultado, null = usa o global. */
  overrides: Record<string, TerminalProfile | null>;
  schemes: ColorScheme[];
  fonts: FontInfo[];
  loaded: boolean;
  load: () => Promise<void>;
  loadOverride: (hostId: string) => Promise<void>;
  /** Atualiza na hora e salva depois de uma pausa (controles deslizantes). */
  updateGlobal: (patch: Partial<TerminalProfile>) => void;
  reloadSchemes: () => Promise<void>;
  /** Grava (ou remove, com null) o perfil próprio de um host; devolve se deu certo. */
  setOverride: (hostId: string, profile: TerminalProfile | null) => Promise<boolean>;
}

const SAVE_DELAY_MS = 250;
let pending: Partial<TerminalProfile> = {};
let saveTimer: ReturnType<typeof setTimeout> | null = null;

const api = () => window.openterminal.profiles;

export const useProfiles = create<ProfilesState>((set, get) => ({
  global: DEFAULT_PROFILE,
  overrides: {},
  schemes: [BRAND_SCHEME],
  fonts: [],
  loaded: false,
  async load() {
    const [global, schemes, fonts] = await Promise.all([
      api().getGlobal(),
      api().listSchemes(),
      api().listFonts(),
    ]);
    set({
      ...(global.ok ? { global: global.value } : {}),
      ...(schemes.ok && schemes.value.length > 0 ? { schemes: schemes.value } : {}),
      ...(fonts.ok ? { fonts: fonts.value } : {}),
      loaded: true,
    });
  },
  async loadOverride(hostId) {
    const res = await api().getForHost(hostId);
    if (res.ok) set({ overrides: { ...get().overrides, [hostId]: res.value } });
  },
  updateGlobal(patch) {
    set({ global: { ...get().global, ...patch } });
    pending = { ...pending, ...patch };
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      const send = pending;
      pending = {};
      saveTimer = null;
      void api()
        .updateGlobal(send)
        .then((res) => {
          if (res.ok) set({ global: { ...res.value, ...pending } });
          else toastError(res.error);
        });
    }, SAVE_DELAY_MS);
  },
  async reloadSchemes() {
    const res = await api().listSchemes();
    if (res.ok && res.value.length > 0) set({ schemes: res.value });
  },
  async setOverride(hostId, profile) {
    const res = await api().setHostOverride(hostId, profile);
    if (!res.ok) {
      toastError(res.error);
      return false;
    }
    set({ overrides: { ...get().overrides, [hostId]: res.value } });
    return true;
  },
}));

let subscribed = false;

export async function subscribeProfiles(): Promise<void> {
  if (subscribed) return;
  subscribed = true;
  api().onChanged((e) => {
    const state = useProfiles.getState();
    if (e.scope === 'host' && e.hostId) void state.loadOverride(e.hostId);
    // Mudança feita por esta janela ainda pendente: o valor local é o mais novo.
    else if (!saveTimer) void state.load();
  });
  await useProfiles.getState().load();
}

/** Perfil efetivo de uma aba: override do host, se houver, senão o global. */
export function effectiveProfile(
  hostId: string | null,
  global: TerminalProfile,
  overrides: Record<string, TerminalProfile | null>,
): TerminalProfile {
  return (hostId ? overrides[hostId] : null) ?? global;
}
