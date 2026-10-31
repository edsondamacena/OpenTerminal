import { t } from '../../i18n/pt-BR';
import { useSessions, type Tab } from '../../stores/sessions';
import { useVault } from '../../stores/vault';
import { toastError } from '../../ui/Toast';
import { focusUnlock } from '../vault/UnlockScreen';
import { stopAutoReconnect } from './autoReconnect';

/** Reconecta a aba SSH; com o cofre bloqueado, leva ao desbloqueio. */
export async function reconnect(tab: Tab): Promise<void> {
  stopAutoReconnect(tab.id);
  if (useVault.getState().status?.state !== 'unlocked') {
    toastError(t.terminal.unlockToReconnect);
    focusUnlock();
    return;
  }
  useSessions.getState().setState(tab.id, 'connecting', null, null);
  const res = await window.openterminal.sessions.reconnect(tab.id);
  if (!res.ok) toastError(res.error);
}
