import { create } from 'zustand';
import type { ActiveForward, ForwardRuleStatus } from '@shared/model/types';

interface ForwardsState {
  /** Regras não inativas (visíveis mesmo com o cofre bloqueado). */
  active: ActiveForward[];
  /** Último status conhecido por regra. */
  statuses: Record<string, ForwardRuleStatus>;
  refreshActive: () => Promise<void>;
}

export const useForwards = create<ForwardsState>((set) => ({
  active: [],
  statuses: {},
  async refreshActive() {
    const res = await window.openterminal.forwards.listActive();
    if (res.ok) set({ active: res.value });
  },
}));

let subscribed = false;

export function subscribeForwards(): void {
  if (subscribed) return;
  subscribed = true;
  window.openterminal.forwards.onStatus(({ ruleId, status, reason }) => {
    useForwards.setState((st) => ({ statuses: { ...st.statuses, [ruleId]: { status, reason } } }));
    void useForwards.getState().refreshActive();
  });
  void useForwards.getState().refreshActive();
}
