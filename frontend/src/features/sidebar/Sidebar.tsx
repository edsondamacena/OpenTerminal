// Barra lateral recolhível (FR-167): lista completa ou trilho de ícones de 56 px, alternada pelo
// botão na barra de título ou por Ctrl+B. A lista fica montada quando recolhida, para os dados do
// cofre (hosts da paleta e da barra de status) não serem descarregados.
import { useState, type ReactNode } from 'react';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { useSessions } from '../../stores/sessions';
import { useUi } from '../../stores/ui';
import { useVault } from '../../stores/vault';
import { useWorkspace } from '../../stores/workspace';
import {
  FolderOpenIcon,
  ForwardIcon,
  KeyIcon,
  LockIcon,
  ServerIcon,
  SettingsIcon,
} from '../../ui/icons';
import { Popover } from '../../ui/Popover';
import { useMobile } from '../../app/mobile';
import { toggleSidebar } from '../terminal/commands';
import { MobileTerminals } from './MobileTerminals';
import { CredentialsDialog } from '../hosts/CredentialsDialog';
import { HostSidebar } from '../hosts/HostSidebar';
import { openSftp } from '../sftp/openSftp';
import { focusUnlock, UnlockPanel } from '../vault/UnlockScreen';
import s from './sidebar.module.css';

/** Botão do trilho com rótulo flutuante à direita no hover e no foco. */
function RailButton({
  label,
  onClick,
  disabled,
  children,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  children: ReactNode;
}) {
  const [tip, setTip] = useState<HTMLElement | null>(null);
  const show = (e: { currentTarget: HTMLElement }) => {
    setTip(e.currentTarget);
  };
  const hide = () => {
    setTip(null);
  };
  return (
    <>
      <button
        type="button"
        className={s.railButton}
        aria-label={label}
        disabled={disabled}
        onClick={() => {
          hide();
          onClick();
        }}
        onMouseEnter={show}
        onMouseLeave={hide}
        onFocus={show}
        onBlur={hide}
      >
        {children}
      </button>
      {tip && (
        <Popover anchor={tip} placement="right" gap={8}>
          {({ ref: floating, style }) => (
            <span ref={floating} style={style} className={`${s.tooltip} raised`} role="tooltip">
              {label}
            </span>
          )}
        </Popover>
      )}
    </>
  );
}

/** Expande e foca um campo da barra depois que a lista volta a aparecer. */
function expandThen(focus: () => void) {
  toggleSidebar();
  requestAnimationFrame(() => {
    requestAnimationFrame(focus);
  });
}

function Rail() {
  const unlocked = useVault((v) => v.status?.state === 'unlocked');
  const sshHostId = useSessions((st) => {
    const tab = st.tabs.find((x) => x.id === st.activeId);
    return tab?.kind === 'ssh' ? tab.hostId : null;
  });
  const sftpHost = useHosts((st) => st.hosts.find((h) => h.id === sshHostId) ?? null);
  const setForwardsOpen = useWorkspace((w) => w.setForwardsOpen);
  const setView = useWorkspace((w) => w.setView);
  const [credentials, setCredentials] = useState(false);

  return (
    <nav className={s.rail} aria-label={t.sidebar.railLabel}>
      {unlocked ? (
        <RailButton
          label={t.sidebar.hosts}
          onClick={() => {
            expandThen(() => {
              document.querySelector<HTMLInputElement>('input[type="search"]')?.focus();
            });
          }}
        >
          <ServerIcon />
        </RailButton>
      ) : (
        <RailButton
          label={t.sidebar.unlock}
          onClick={() => {
            expandThen(focusUnlock);
          }}
        >
          <LockIcon />
        </RailButton>
      )}
      <RailButton
        label={unlocked ? t.sidebar.credentials : t.sidebar.credentialsLocked}
        disabled={!unlocked}
        onClick={() => {
          setCredentials(true);
        }}
      >
        <KeyIcon />
      </RailButton>
      <RailButton
        label={sftpHost ? t.sidebar.sftp(sftpHost.label) : t.sidebar.sftpUnavailable}
        disabled={!sftpHost}
        onClick={() => {
          if (sftpHost) void openSftp(sftpHost);
        }}
      >
        <FolderOpenIcon />
      </RailButton>
      <RailButton
        label={t.sidebar.forwards}
        onClick={() => {
          setForwardsOpen(true);
        }}
      >
        <ForwardIcon />
      </RailButton>
      <span className={s.spacer} />
      <RailButton
        label={t.settings.title}
        onClick={() => {
          setView('settings');
        }}
      >
        <SettingsIcon />
      </RailButton>
      {credentials && (
        <CredentialsDialog
          onClose={() => {
            setCredentials(false);
          }}
        />
      )}
    </nav>
  );
}

export function Sidebar() {
  const mobile = useMobile();
  // No celular a lista ocupa a tela inteira: o trilho recolhido não faz sentido.
  const collapsed = useUi((u) => u.settings?.sidebarCollapsed ?? false) && !mobile;
  const unlocked = useVault((v) => v.status?.state === 'unlocked');

  return (
    <div className={s.sidebar} data-collapsed={collapsed || undefined}>
      <div className={s.full} hidden={collapsed}>
        {mobile && unlocked && <MobileTerminals />}
        <div className={s.content}>{unlocked ? <HostSidebar /> : <UnlockPanel />}</div>
      </div>
      {collapsed && <Rail />}
    </div>
  );
}
