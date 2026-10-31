import { create } from 'zustand';
import type { Result } from '@shared/ipc/result';
import type { VaultSettings, VaultStatus } from '@shared/model/types';

interface VaultStoreState {
  status: VaultStatus | null;
  settings: VaultSettings | null;
  refresh: () => Promise<void>;
  create: (password: string, confirmation: string) => Promise<Result<void>>;
  unlock: (password: string) => Promise<Result<void>>;
  lock: () => Promise<void>;
  changePassword: (current: string, next: string, confirmation: string) => Promise<Result<void>>;
  erase: (phrase: string) => Promise<Result<void>>;
  loadSettings: () => Promise<void>;
  updateSettings: (patch: Partial<VaultSettings>) => Promise<Result<VaultSettings>>;
}

const api = () => window.openterminal.vault;

export const useVault = create<VaultStoreState>((set, get) => ({
  status: null,
  settings: null,
  async refresh() {
    const res = await api().getStatus();
    if (res.ok) set({ status: res.value });
  },
  async create(password, confirmation) {
    const res = await api().create(password, confirmation);
    await get().refresh();
    return res;
  },
  async unlock(password) {
    const res = await api().unlock(password);
    await get().refresh();
    return res;
  },
  async lock() {
    await api().lock();
    await get().refresh();
  },
  changePassword(current, next, confirmation) {
    return api().changePassword(current, next, confirmation);
  },
  async erase(phrase) {
    const res = await api().erase(phrase);
    await get().refresh();
    return res;
  },
  async loadSettings() {
    const res = await api().getSettings();
    if (res.ok) set({ settings: res.value });
  },
  async updateSettings(patch) {
    const res = await api().updateSettings(patch);
    if (res.ok) set({ settings: res.value });
    return res;
  },
}));

let subscribed = false;

export function subscribeVault(): void {
  if (subscribed) return;
  subscribed = true;
  api().onStateChanged((status) => {
    useVault.setState({ status, ...(status.state === 'unlocked' ? {} : { settings: null }) });
  });
}
