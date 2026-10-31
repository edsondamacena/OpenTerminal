import type { HostView } from '@shared/model/types';
import { useSessions } from '../../stores/sessions';
import { toastError } from '../../ui/Toast';

export async function connectHost(host: HostView): Promise<void> {
  const res = await window.openterminal.sessions.openSsh(host.id);
  if (!res.ok) {
    toastError(res.error);
    return;
  }
  useSessions.getState().add({
    id: res.value.sessionId,
    kind: host.protocol === 'telnet' ? 'telnet' : 'ssh',
    title: host.label,
    hostId: host.id,
    shell: null,
    state: 'connecting',
  });
}
