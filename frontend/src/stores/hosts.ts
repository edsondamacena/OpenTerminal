import { create } from 'zustand';
import type { CredentialSummary, Group, HostView } from '@shared/model/types';
import { toastError } from '../ui/Toast';

interface HostsState {
  hosts: HostView[];
  groups: Group[];
  credentials: CredentialSummary[];
  loaded: boolean;
  reload: () => Promise<void>;
  clear: () => void;
}

export const useHosts = create<HostsState>((set) => ({
  hosts: [],
  groups: [],
  credentials: [],
  loaded: false,
  async reload() {
    const [hosts, groups, credentials] = await Promise.all([
      window.openterminal.hosts.list(),
      window.openterminal.groups.list(),
      window.openterminal.credentials.list(),
    ]);
    if (!hosts.ok) {
      toastError(hosts.error);
      return;
    }
    if (!groups.ok) {
      toastError(groups.error);
      return;
    }
    if (!credentials.ok) {
      toastError(credentials.error);
      return;
    }
    set({ hosts: hosts.value, groups: groups.value, credentials: credentials.value, loaded: true });
  },
  clear() {
    // Ao bloquear, nada do cofre fica na interface (FR-003).
    set({ hosts: [], groups: [], credentials: [], loaded: false });
  },
}));
