// Barra de status (FR-172): aba ativa, estado, latência, e túneis (o cofre fica no cadeado da barra de título); cada item leva à ação.
import type { SessionState } from '@shared/model/types';
import { focusTerminal } from '../features/terminal/terminalRegistry';
import { reconnect } from '../features/terminal/reconnect';
import { t } from '../i18n/pt-BR';
import { useForwards } from '../stores/forwards';
import { useHosts } from '../stores/hosts';
import { useSessions } from '../stores/sessions';
import { useWorkspace } from '../stores/workspace';
import { ForwardIcon, ServerIcon, TerminalIcon } from '../ui/icons';
import { latencyLevel } from './latency';
import s from './statusbar.module.css';

function stateText(state: SessionState, local: boolean): string {
  return local && state === 'connected' ? t.status.running : t.status.states[state];
}

export function StatusBar() {
  const tab = useSessions((st) => st.tabs.find((x) => x.id === st.activeId) ?? null);
  const host = useHosts((st) =>
    tab?.hostId ? (st.hosts.find((h) => h.id === tab.hostId) ?? null) : null,
  );
  const running = useForwards((st) => st.active.filter((a) => a.status === 'active').length);
  const anyForward = useForwards((st) => st.active.length > 0);
  const setForwardsOpen = useWorkspace((w) => w.setForwardsOpen);

  const local = tab?.kind === 'local';
  const canReconnect =
    tab !== null && !local && (tab.state === 'disconnected' || tab.state === 'error');
  const address = host ? `${host.username}@${host.address}` : null;

  return (
    <footer className={s.bar} aria-label={t.status.label}>
      {tab ? (
        <>
          <button
            type="button"
            className={s.item}
            title={address ?? tab.title}
            data-testid="status-tab"
            onClick={() => {
              focusTerminal(tab.id);
            }}
          >
            {local ? <TerminalIcon /> : <ServerIcon />}
            <span className={s.text}>{tab.title}</span>
            {address && <span className={s.muted}>{address}</span>}
          </button>
          <button
            type="button"
            className={s.item}
            disabled={!canReconnect}
            title={canReconnect ? t.status.reconnect : undefined}
            data-testid="status-state"
            onClick={() => {
              if (canReconnect) void reconnect(tab);
            }}
          >
            <span className={s.dot} data-state={tab.state} aria-hidden />
            {stateText(tab.state, local)}
          </button>
          {tab.latencyMs !== null && tab.state === 'connected' && (
            <span
              className={s.latency}
              data-level={latencyLevel(tab.latencyMs)}
              title={t.status.latencyTitle}
              data-testid="status-latency"
            >
              {t.status.latency(tab.latencyMs)}
            </span>
          )}
        </>
      ) : (
        <span className={`${s.muted} ${s.pad}`}>{t.status.noTab}</span>
      )}
      <span className={s.spacer} />
      {anyForward && (
        <button
          type="button"
          className={s.item}
          title={t.status.forwardsTitle}
          aria-label={t.forwarding.activeCount(running)}
          onClick={() => {
            setForwardsOpen(true);
          }}
        >
          <ForwardIcon />
          {t.status.forwards(running)}
        </button>
      )}
    </footer>
  );
}
