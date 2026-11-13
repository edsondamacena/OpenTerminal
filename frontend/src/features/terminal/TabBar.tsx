import { useRef, useState, type ReactNode } from 'react';
import { t } from '../../i18n/pt-BR';
import type { SendKeysCombo } from '@shared/model/types';
import { isLive, useSessions, type Tab } from '../../stores/sessions';
import { useWorkspace } from '../../stores/workspace';
import { Button } from '../../ui/Button';
import { useSlidingIndicator } from '../../ui/motion';
import {
  CloseIcon,
  ConnectedIcon,
  ConnectingIcon,
  DisconnectedIcon,
  ErrorIcon,
  ExitedIcon,
  KeyboardIcon,
  PlusIcon,
  RenameIcon,
} from '../../ui/icons';
import { Menu, type MenuItem } from '../../ui/Menu';
import { toastError } from '../../ui/Toast';
import { closeActiveTab, requestCloseTab } from './commands';
import s from './terminal.module.css';

function stateLabel(tab: Tab): string {
  switch (tab.state) {
    case 'connected':
      return tab.title;
    case 'disconnected':
      return `${tab.title} — ${t.terminal.disconnected}`;
    case 'closed':
      return `${tab.title} — ${t.terminal.exited(tab.exitCode)}`;
    default:
      return `${tab.title} — ${t.terminal.connecting}`;
  }
}

/** Estado da aba por cor e por forma (FR-163): quem não distingue as cores vê o ícone. */
function StateIcon({ state }: { state: Tab['state'] }) {
  const Icon =
    state === 'connected'
      ? ConnectedIcon
      : state === 'disconnected'
        ? DisconnectedIcon
        : state === 'error'
          ? ErrorIcon
          : state === 'closed'
            ? ExitedIcon
            : ConnectingIcon;
  return (
    <span className={s.state} data-state={state} aria-hidden>
      <Icon width={13} height={13} />
    </span>
  );
}

/** Combinações que o app usa como atalho; o menu da aba as envia ao shell (FR-140). */
const SEND_KEYS: [SendKeysCombo, string][] = [
  ['ctrl+w', 'Ctrl+W'],
  ['ctrl+t', 'Ctrl+T'],
  ['ctrl+tab', 'Ctrl+Tab'],
  ['ctrl+k', 'Ctrl+K'],
  ['ctrl+b', 'Ctrl+B'],
];

function tabMenu(tab: Tab, startRename: () => void): MenuItem[] {
  const items: MenuItem[] = [
    { label: t.terminal.renameTab, icon: <RenameIcon />, onSelect: startRename },
  ];
  if (tab.kind !== 'sftp' && isLive(tab.state))
    for (const [combo, label] of SEND_KEYS)
      items.push({
        label: t.terminal.sendKeys(label),
        icon: <KeyboardIcon />,
        onSelect: () => {
          void window.openterminal.sessions.sendKeys(tab.id, combo).then((res) => {
            if (!res.ok) toastError(res.error);
          });
        },
      });
  items.push({
    label: t.terminal.closeTab,
    icon: <CloseIcon />,
    onSelect: () => {
      useSessions.getState().activate(tab.id);
      closeActiveTab();
    },
  });
  return items;
}

function TabItem({ tab, active }: { tab: Tab; active: boolean }) {
  const { activate, rename, reorder } = useSessions();
  const [editing, setEditing] = useState(false);
  const [dropTarget, setDropTarget] = useState(false);
  const [menuAt, setMenuAt] = useState<{ x: number; y: number } | null>(null);

  return (
    <div
      role="tab"
      aria-selected={active}
      tabIndex={active ? 0 : -1}
      title={stateLabel(tab)}
      className={`${s.tab} ${active ? s.tabActive : ''} ${dropTarget ? s.dropBefore : ''} no-drag`}
      draggable={!editing}
      onClick={() => {
        activate(tab.id);
        useWorkspace.getState().setView('terminal');
      }}
      onDoubleClick={() => {
        setEditing(true);
      }}
      onContextMenu={(e) => {
        e.preventDefault();
        activate(tab.id);
        setMenuAt({ x: e.clientX, y: e.clientY });
      }}
      onAuxClick={(e) => {
        if (e.button === 1) requestCloseTab(tab);
      }}
      onDragStart={(e) => {
        e.dataTransfer.setData('application/x-openterminal-tab', tab.id);
        e.dataTransfer.effectAllowed = 'move';
      }}
      onDragOver={(e) => {
        if (e.dataTransfer.types.includes('application/x-openterminal-tab')) {
          e.preventDefault();
          setDropTarget(true);
        }
      }}
      onDragLeave={() => {
        setDropTarget(false);
      }}
      onDrop={(e) => {
        setDropTarget(false);
        const id = e.dataTransfer.getData('application/x-openterminal-tab');
        if (id) reorder(id, tab.id);
      }}
    >
      <StateIcon state={tab.state} />
      {editing ? (
        <input
          className={s.tabRename}
          defaultValue={tab.title}
          aria-label={t.terminal.renameTab}
          autoFocus
          onFocus={(e) => {
            e.currentTarget.select();
          }}
          onBlur={(e) => {
            rename(tab.id, e.currentTarget.value);
            setEditing(false);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') e.currentTarget.blur();
            if (e.key === 'Escape') setEditing(false);
          }}
        />
      ) : (
        <span className={s.tabTitle}>{tab.title}</span>
      )}
      <Button
        variant="ghost"
        iconOnly
        className={s.tabClose}
        aria-label={`${t.terminal.closeTab} ${tab.title}`}
        onClick={(e) => {
          e.stopPropagation();
          requestCloseTab(tab);
        }}
      >
        <CloseIcon width={12} height={12} />
      </Button>
      {menuAt && (
        <Menu
          anchor={menuAt}
          items={tabMenu(tab, () => {
            setEditing(true);
          })}
          onClose={() => {
            setMenuAt(null);
          }}
        />
      )}
    </div>
  );
}

export interface TabBarProps {
  /** Itens do menu "+" (ex.: terminal local); vazio esconde o botão. */
  newTabItems: MenuItem[];
  trailing?: ReactNode;
}

export function TabBar({ newTabItems, trailing }: TabBarProps) {
  const { tabs, activeId, reorder } = useSessions();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const barRef = useRef<HTMLDivElement>(null);
  // Ordem e títulos mudam a posição da aba ativa sem trocar o id: entram na chave.
  const layoutKey = `${activeId ?? ''}|${tabs.map((tb) => `${tb.id}:${tb.title}`).join('|')}`;
  const indicator = useSlidingIndicator(barRef, layoutKey, 'x');

  return (
    <div
      ref={barRef}
      className={s.tabBar}
      role="tablist"
      style={indicator ?? undefined}
      data-indicator={indicator ? '' : undefined}
      onDragOver={(e) => {
        if (e.dataTransfer.types.includes('application/x-openterminal-tab')) e.preventDefault();
      }}
      onDrop={(e) => {
        const id = e.dataTransfer.getData('application/x-openterminal-tab');
        if (id && e.target === e.currentTarget) reorder(id, null);
      }}
    >
      {tabs.map((tab) => (
        <TabItem key={tab.id} tab={tab} active={tab.id === activeId} />
      ))}
      {newTabItems.length > 0 && (
        <Button
          variant="ghost"
          className={`${s.newTab} no-drag`}
          title={newTabItems.length > 1 ? t.terminal.newLocalHint : undefined}
          onClick={() => {
            newTabItems[0]?.onSelect();
          }}
          onContextMenu={(e) => {
            // Botão direito: escolher outro shell.
            if (newTabItems.length < 2) return;
            e.preventDefault();
            const el = e.currentTarget;
            setMenuAnchor((cur) => (cur ? null : el));
          }}
        >
          {t.terminal.newTerminal}
          <PlusIcon />
        </Button>
      )}
      {trailing}
      {menuAnchor && (
        <Menu
          anchor={menuAnchor}
          items={newTabItems}
          onClose={() => {
            setMenuAnchor(null);
          }}
        />
      )}
    </div>
  );
}
