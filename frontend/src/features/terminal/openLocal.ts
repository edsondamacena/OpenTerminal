// Abre uma aba de terminal local (US4); funciona com o cofre bloqueado.
import type { LocalShell } from '@shared/model/types';
import { useSessions } from '../../stores/sessions';
import { shellInfo } from '../../stores/shells';
import { toastError } from '../../ui/Toast';

export async function openLocal(shell: LocalShell): Promise<void> {
  const res = await window.openterminal.sessions.openLocal(shell);
  if (!res.ok) {
    toastError(res.error);
    return;
  }
  useSessions.getState().add({
    id: res.value.sessionId,
    kind: 'local',
    title: shellInfo(shell).label,
    hostId: null,
    shell,
    state: 'connecting',
  });
}
