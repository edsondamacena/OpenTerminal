import { useEffect } from 'react';
import { CommandPalette } from '../features/palette/CommandPalette';
import { ActiveForwardsDialog } from '../features/forwarding/ActiveForwardsDialog';
import { Sidebar } from '../features/sidebar/Sidebar';
import { toggleSidebar } from '../features/terminal/commands';
import { Workspace } from '../features/terminal/Workspace';
import { CreateVaultScreen } from '../features/vault/CreateVaultScreen';
import { focusUnlock } from '../features/vault/UnlockScreen';
import { UnreadableScreen } from '../features/vault/UnreadableScreen';
import { t } from '../i18n/pt-BR';
import { useSessions } from '../stores/sessions';
import { useUi } from '../stores/ui';
import { subscribeVault, useVault } from '../stores/vault';
import { useWorkspace } from '../stores/workspace';
import { Button } from '../ui/Button';
import {
  BackIcon,
  LockIcon,
  SettingsIcon,
  SidebarCollapseIcon,
  SidebarExpandIcon,
  TerminalIcon,
  UnlockIcon,
} from '../ui/icons';
import { useHosts } from '../stores/hosts';
import { useMobile } from './mobile';
import { Toasts } from '../ui/Toast';
import { trackActivity } from './activity';
import { CloseConfirmDialog } from './CloseConfirmDialog';
import { StatusBar } from './StatusBar';
import { TitleBar } from './TitleBar';
import { useShortcutLabel, withShortcut } from './useShortcuts';
import s from './app.module.css';

function FullscreenFrame({ children }: { children: React.ReactNode }) {
  return (
    <div className={`${s.fullscreen} ${s.backdrop} glass`}>
      <TitleBar />
      <div className={s.fullscreenBody}>{children}</div>
    </div>
  );
}

export function App() {
  const { status, refresh, lock } = useVault();
  const view = useWorkspace((w) => w.view);
  const collapsed = useUi((u) => u.settings?.sidebarCollapsed ?? false);
  const sidebarLabel = withShortcut(
    collapsed ? t.sidebar.expand : t.sidebar.collapse,
    useShortcutLabel('toggleSidebar'),
  );
  const setView = useWorkspace((w) => w.setView);
  const mobile = useMobile();
  const mobilePane = useWorkspace((w) => w.mobilePane);
  const setMobilePane = useWorkspace((w) => w.setMobilePane);
  const tabCount = useSessions((st) => st.tabs.length);

  useEffect(() => {
    subscribeVault();
    void refresh();
    return trackActivity(() => void window.openterminal.vault.reportActivity());
  }, [refresh]);

  // Abrir ou trocar de aba (não fechar) leva de volta às abas; no celular, ao console em tela
  // cheia. Sem nenhuma aba, o celular volta para a lista.
  useEffect(
    () =>
      useSessions.subscribe((next, prev) => {
        const workspace = useWorkspace.getState();
        if (next.activeId !== prev.activeId && next.tabs.length >= prev.tabs.length) {
          workspace.setView('terminal');
          workspace.setMobilePane('terminal');
        }
        if (next.tabs.length === 0) workspace.setMobilePane('list');
      }),
    [],
  );

  // O sync gravou dados da nuvem no cofre: recarrega hosts, grupos e credenciais. Começa em 0
  // porque o sync do desbloqueio pode terminar antes desta inscrição; recarregar a mais é barato.
  useEffect(() => {
    let pulls = 0;
    return window.openterminal.sync.onStatus((st) => {
      if (st.pulls !== pulls) void useHosts.getState().reload();
      pulls = st.pulls;
    });
  }, []);

  // Voltou ao app (Android: onResume; PC: janela visível de novo): busca novidades da nuvem na
  // hora, sem esperar o ciclo de 3 minutos, que o Android congela em segundo plano.
  useEffect(() => {
    let last = 0;
    const resume = () => {
      if (document.visibilityState === 'hidden' || Date.now() - last < 15_000) return;
      last = Date.now();
      void window.openterminal.sync.syncNow().catch(() => undefined);
    };
    const w = window as Window & { openterminalResume?: () => void };
    w.openterminalResume = resume;
    document.addEventListener('visibilitychange', resume);
    return () => {
      delete w.openterminalResume;
      document.removeEventListener('visibilitychange', resume);
    };
  }, []);

  // Botão voltar do Android: no console, volta para a lista; senão o app vai para segundo plano.
  useEffect(() => {
    const w = window as Window & { openterminalBack?: () => boolean };
    w.openterminalBack = () => {
      const workspace = useWorkspace.getState();
      if (workspace.view === 'settings') {
        workspace.setView('terminal');
        return true;
      }
      if (workspace.mobilePane === 'terminal') {
        workspace.setMobilePane('list');
        return true;
      }
      return false;
    };
    return () => {
      delete w.openterminalBack;
    };
  }, []);

  // Cofre apagado: ao criar o novo, começa pelas abas.
  useEffect(() => {
    if (status?.state === 'nonexistent') setView('terminal');
  }, [status?.state, setView]);

  if (!status) return null;

  if (status.state === 'nonexistent') {
    return (
      <FullscreenFrame>
        <CreateVaultScreen />
        <Toasts />
      </FullscreenFrame>
    );
  }
  if (status.state === 'unreadable') {
    return (
      <FullscreenFrame>
        <UnreadableScreen />
        <Toasts />
      </FullscreenFrame>
    );
  }

  const unlocked = status.state === 'unlocked';
  return (
    <div
      className={`${s.root} ${s.backdrop} glass`}
      data-sidebar={collapsed ? 'rail' : 'full'}
      data-mobile={mobile || undefined}
      data-pane={view === 'settings' ? 'terminal' : mobilePane}
    >
      <TitleBar
        leading={
          mobile ? (
            mobilePane === 'terminal' || view === 'settings' ? (
              <Button
                variant="ghost"
                iconOnly
                aria-label={t.sidebar.backToList}
                title={t.sidebar.backToList}
                onClick={() => {
                  setView('terminal');
                  setMobilePane('list');
                }}
              >
                <BackIcon />
              </Button>
            ) : tabCount > 0 ? (
              <Button
                variant="ghost"
                iconOnly
                aria-label={t.sidebar.showTerminal(tabCount)}
                title={t.sidebar.showTerminal(tabCount)}
                onClick={() => {
                  setMobilePane('terminal');
                }}
              >
                <TerminalIcon />
              </Button>
            ) : null
          ) : (
            <Button
              variant="ghost"
              iconOnly
              aria-label={sidebarLabel}
              title={sidebarLabel}
              aria-expanded={!collapsed}
              onClick={toggleSidebar}
            >
              {collapsed ? <SidebarExpandIcon /> : <SidebarCollapseIcon />}
            </Button>
          )
        }
        actions={
          <>
            {/* O cadeado é o único controle do cofre: aberto, bloqueia; bloqueado, leva à senha. */}
            {(unlocked || status.state === 'locked') && (
              <Button
                variant="ghost"
                iconOnly
                aria-label={unlocked ? t.vault.lock : t.status.unlockTitle}
                title={unlocked ? t.vault.lock : t.status.unlockTitle}
                onClick={() => {
                  if (unlocked) {
                    void lock();
                    return;
                  }
                  setView('terminal');
                  setMobilePane('list');
                  focusUnlock();
                }}
              >
                {unlocked ? <LockIcon /> : <UnlockIcon />}
              </Button>
            )}
            <Button
              variant="ghost"
              iconOnly
              aria-label={t.settings.title}
              title={t.settings.title}
              aria-pressed={view === 'settings'}
              onClick={() => {
                setView(view === 'settings' ? 'terminal' : 'settings');
              }}
            >
              <SettingsIcon />
            </Button>
          </>
        }
      />
      {/* Bloqueado: o painel de desbloqueio ocupa só a barra lateral; abas seguem ativas. */}
      <aside className={s.sidebar}>
        <Sidebar />
      </aside>
      <main className={s.main}>
        <Workspace />
      </main>
      <StatusBar />
      <ActiveForwardsDialog />
      <CommandPalette />
      <CloseConfirmDialog />
      <Toasts />
    </div>
  );
}
