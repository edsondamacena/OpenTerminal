// Abre uma aba SFTP para o host (US5); a conexão passa pelo mesmo fluxo de host key/prompts.
import type { HostView } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useSessions } from '../../stores/sessions';
import { toastError } from '../../ui/Toast';

const homes = new Map<string, string>();

/** Pasta inicial da aba SFTP (home do usuário remoto). */
export function homeOf(sessionId: string): string {
  return homes.get(sessionId) ?? '/';
}

export async function openSftp(host: HostView): Promise<void> {
  const res = await window.openterminal.sftp.open(host.id);
  if (!res.ok) {
    toastError(res.error);
    return;
  }
  homes.set(res.value.sessionId, res.value.homePath);
  useSessions.getState().add({
    id: res.value.sessionId,
    kind: 'sftp',
    title: t.sftp.tabTitle(host.label),
    hostId: host.id,
    shell: null,
    state: 'connected',
  });
}
