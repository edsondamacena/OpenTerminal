import { useEffect, useMemo, useState } from 'react';
import { installShortcuts } from '../../app/shortcuts';
import { currentKeymap, useShortcutLabel } from '../../app/useShortcuts';
import { t } from '../../i18n/pt-BR';
import { closeTab, isLive, useSessions, type Tab } from '../../stores/sessions';
import { useShells } from '../../stores/shells';
import { useUi } from '../../stores/ui';
import { useVault } from '../../stores/vault';
import { useWorkspace } from '../../stores/workspace';
import { ConfirmDialog } from '../../ui/ConfirmDialog';
import { Button } from '../../ui/Button';
import { DisconnectedIcon, ErrorIcon, ExitedIcon, RefreshIcon, TerminalIcon } from '../../ui/icons';
import { ShellIcon } from '../../ui/ShellIcon';
import { ConflictDialog } from '../sftp/ConflictDialog';
import { SettingsPage } from '../settings/SettingsPage';
import { SftpView } from '../sftp/SftpView';
import { MAX_ATTEMPTS, stopAutoReconnect, useAutoReconnect } from './autoReconnect';
import { runShortcut } from './commands';
import { HostKeyDialog } from './HostKeyDialog';
import { openLocal } from './openLocal';
import { reconnect } from './reconnect';
import { PasteConfirmDialog } from './PasteConfirmDialog';
import { PromptDialog } from './PromptDialog';
import { TabBar } from './TabBar';
import { TerminalView } from './TerminalView';
import s from './terminal.module.css';
import app from '../../app/app.module.css';

/** Segundos até a próxima tentativa automática, atualizados a cada segundo. */
function useSecondsUntil(at: number | undefined): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (at === undefined) return;
    const timer = setInterval(() => {
      setNow(Date.now());
    }, 1_000);
    return () => {
      clearInterval(timer);
    };
  }, [at]);
  return at === undefined ? 0 : Math.max(0, Math.ceil((at - now) / 1_000));
}

function BannerIcon({ state }: { state: Tab['state'] }) {
  if (state === 'closed') return <ExitedIcon className={s.bannerIcon} aria-hidden />;
  const Icon = state === 'error' ? ErrorIcon : DisconnectedIcon;
  return <Icon className={`${s.bannerIcon} ${s.bannerIconDanger}`} aria-hidden />;
}

function Banner({ tab }: { tab: Tab }) {
  const pending = useAutoReconnect((r) => r.pending[tab.id]);
  const seconds = useSecondsUntil(pending?.at);
  if (pending && (tab.state === 'disconnected' || tab.state === 'error')) {
    return (
      <div className={s.banner} role="status">
        <BannerIcon state={tab.state} />
        <span className={s.bannerText}>
          {t.terminal.reconnectIn(seconds, pending.attempt, MAX_ATTEMPTS)}
          {tab.reason ? ` — ${tab.reason}` : ''}
        </span>
        <Button
          onClick={() => {
            stopAutoReconnect(tab.id);
          }}
        >
          {t.terminal.reconnectCancel}
        </Button>
        <Button variant="primary" onClick={() => void reconnect(tab)}>
          <RefreshIcon />
          {t.terminal.reconnect}
        </Button>
      </div>
    );
  }
  if (tab.state === 'disconnected' || tab.state === 'error') {
    return (
      <div className={s.banner} role="status">
        <BannerIcon state={tab.state} />
        <span className={s.bannerText}>
          {t.terminal.disconnected}
          {tab.reason ? ` — ${tab.reason}` : ''}
        </span>
        {(tab.kind === 'ssh' || tab.kind === 'telnet') && (
          <Button variant="primary" onClick={() => void reconnect(tab)}>
            <RefreshIcon />
            {t.terminal.reconnect}
          </Button>
        )}
      </div>
    );
  }
  if (tab.state === 'closed') {
    return (
      <div className={s.banner} role="status">
        <BannerIcon state={tab.state} />
        <span className={s.bannerText}>{t.terminal.exited(tab.exitCode)}</span>
        <Button onClick={() => void closeTab(tab.id)}>{t.terminal.closeTab}</Button>
      </div>
    );
  }
  return null;
}

function CloseTabConfirm() {
  const id = useWorkspace((w) => w.confirmClose);
  const setConfirmClose = useWorkspace((w) => w.setConfirmClose);
  const tab = useSessions((st) => st.tabs.find((tb) => tb.id === id));
  if (!id || !tab) return null;
  return (
    <ConfirmDialog
      title={t.terminal.closeConnected.title}
      message={t.terminal.closeConnected.message(tab.title)}
      confirmLabel={t.terminal.closeConnected.confirm}
      danger
      onCancel={() => {
        setConfirmClose(null);
      }}
      onConfirm={() => {
        setConfirmClose(null);
        void closeTab(id);
      }}
    />
  );
}

let autoOpened = false;

export function Workspace() {
  const { tabs, activeId } = useSessions();
  const settings = useWorkspace((w) => w.view === 'settings');
  const newTabShortcut = useShortcutLabel('newTab');
  const shells = useShells((s) => s.shells);
  const defaultShell = useUi((u) => u.settings?.defaultShell);
  const newLocal = useMemo(
    () =>
      // O shell padrão vem primeiro: é o que o botão "Novo terminal" abre direto.
      [...shells]
        .sort((x, y) => Number(y.id === defaultShell) - Number(x.id === defaultShell))
        .map((shell) => ({
        label: shell.label,
        icon: <ShellIcon kind={shell.kind} label={shell.label} />,
        onSelect: () => void openLocal(shell.id),
      })),
    [shells, defaultShell],
  );

  // Atalhos de FR-140/FR-173, capturados antes do xterm para não virarem caracteres de controle.
  useEffect(() => installShortcuts(runShortcut, currentKeymap), []);
  // Abriu sem pedir a senha (cofre já aberto) e sem abas: começa direto num terminal local
  // com o shell padrão. Uma vez por carga da página; espera um pouco para as abas de sessões
  // vivas voltarem (janela reaberta) antes de decidir.
  const firstShell = newLocal[0];
  const vaultState = useVault((v) => v.status?.state);
  useEffect(() => {
    if (autoOpened || !firstShell || vaultState !== 'unlocked') return;
    if (/Android/.test(navigator.userAgent)) return;
    const timer = window.setTimeout(() => {
      autoOpened = true;
      if (useSessions.getState().tabs.length === 0) firstShell.onSelect();
    }, 600);
    return () => {
      window.clearTimeout(timer);
    };
  }, [firstShell, vaultState]);
  // Fechar a janela (Alt+F4, X) com sessão aberta pergunta antes; o navegador mostra o aviso dele.
  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!useSessions.getState().tabs.some((tab) => isLive(tab.state))) return;
      e.preventDefault();
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => {
      window.removeEventListener('beforeunload', onBeforeUnload);
    };
  }, []);

  return (
    <>
      <TabBar newTabItems={newLocal} />
      <div className={`${app.content} solid`} data-terminal-area>
        {/* Sob a página de configurações as abas ficam escondidas, não desmontadas:
            sessões e tamanho do xterm seguem intactos (FR-168). */}
        <div className={app.workspaceLayer} inert={settings}>
          {tabs.length === 0 && (
            <div className={app.empty}>
              <span className={app.emptyIcon}>
                <TerminalIcon width={28} height={28} />
              </span>
              <p className={app.emptyTitle}>{t.terminal.emptyTitle}</p>
              <p className={app.emptyHint}>{t.terminal.emptyHint(newTabShortcut)}</p>
              {defaultShell && (
                <Button
                  variant="primary"
                  className={app.emptyAction}
                  onClick={() => void openLocal(defaultShell)}
                >
                  <TerminalIcon />
                  {t.terminal.emptyAction}
                </Button>
              )}
            </div>
          )}
          {tabs.map((tab) =>
            tab.kind === 'sftp' ? (
              <SftpView key={tab.id} tab={tab} active={tab.id === activeId} />
            ) : (
              <TerminalView
                key={tab.id}
                tab={tab}
                active={tab.id === activeId}
                banner={<Banner tab={tab} />}
              />
            ),
          )}
        </div>
        {settings && <SettingsPage />}
      </div>
      <HostKeyDialog />
      <PromptDialog />
      <PasteConfirmDialog />
      <CloseTabConfirm />
      <ConflictDialog />
    </>
  );
}
