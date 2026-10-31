// Paleta de comandos (Ctrl+Shift+P, FR-141): busca e executa ações e conexões só pelo teclado.
import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { useShells } from '../../stores/shells';
import { useVault } from '../../stores/vault';
import { useWorkspace } from '../../stores/workspace';
import {
  FolderIcon,
  LockIcon,
  SearchIcon,
  ServerIcon,
  SettingsIcon,
  ThemeIcon,
} from '../../ui/icons';
import { connectHost } from '../hosts/connect';
import { openSftp } from '../sftp/openSftp';
import { activeTerminalId, openSearch, toggleTheme } from '../terminal/commands';
import { openLocal } from '../terminal/openLocal';
import { useShortcutLabel } from '../../app/useShortcuts';
import { ShellIcon } from '../../ui/ShellIcon';
import { filterCommands, type Command } from './palette';
import s from './palette.module.css';

/** Itens mostrados de uma vez; a busca estreita o resto (1.000 hosts viram 2.000 itens). */
const LIMIT = 50;

type Entry = Command & { icon: ReactNode };

function useCommands(): Entry[] {
  const unlocked = useVault((v) => v.status?.state === 'unlocked');
  const hosts = useHosts((h) => h.hosts);
  const lock = useVault((v) => v.lock);
  const setView = useWorkspace((w) => w.setView);
  const powershellKey = useShortcutLabel('newPowerShell') ?? '';
  const shells = useShells((x) => x.shells);
  const searchKey = useShortcutLabel('search') ?? '';

  return useMemo(() => {
    const p = t.palette;
    const actions: Entry[] = shells.map((shell) => ({
      id: `local:${shell.id}`,
      label: p.newLocal(shell.label),
      hint: shell.id === 'powershell' ? powershellKey : '',
      group: 'action',
      icon: <ShellIcon kind={shell.kind} label={shell.label} />,
      run: () => void openLocal(shell.id),
    }));
    if (activeTerminalId())
      actions.push({
        id: 'search',
        label: p.search,
        hint: searchKey,
        group: 'action',
        icon: <SearchIcon />,
        run: openSearch,
      });
    actions.push(
      {
        id: 'settings',
        label: p.settings,
        hint: '',
        group: 'action',
        icon: <SettingsIcon />,
        run: () => {
          setView('settings');
        },
      },
      {
        id: 'theme',
        label: p.toggleTheme,
        hint: '',
        group: 'action',
        icon: <ThemeIcon />,
        run: () => void toggleTheme(),
      },
    );
    if (!unlocked) return actions;
    actions.push({
      id: 'lock',
      label: p.lock,
      hint: '',
      group: 'action',
      icon: <LockIcon />,
      run: () => void lock(),
    });
    const hostEntries: Entry[] = hosts.flatMap((host) => {
      const hint = `${host.username}@${host.address}`;
      return [
        {
          id: `ssh:${host.id}`,
          label: p.connect(host.label),
          hint,
          group: 'host' as const,
          icon: <ServerIcon />,
          run: () => void connectHost(host),
        },
        {
          id: `sftp:${host.id}`,
          label: p.sftp(host.label),
          hint,
          group: 'host' as const,
          icon: <FolderIcon />,
          run: () => void openSftp(host),
        },
      ];
    });
    return [...actions, ...hostEntries];
  }, [shells, unlocked, hosts, lock, setView, powershellKey, searchKey]);
}

export function CommandPalette() {
  const open = useWorkspace((w) => w.paletteOpen);
  return open ? <Palette /> : null;
}

function Palette() {
  const setOpen = useWorkspace((w) => w.setPaletteOpen);
  const commands = useCommands();
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(0);
  const listId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const previous = useRef<HTMLElement | null>(null);

  const matches = useMemo(() => filterCommands(commands, query), [commands, query]);
  const shown = matches.slice(0, LIMIT);
  const current = Math.min(selected, Math.max(0, shown.length - 1));

  useEffect(() => {
    previous.current = document.activeElement as HTMLElement | null;
    inputRef.current?.focus();
    return () => {
      // Sem comando executado, o foco volta para onde estava (em geral o terminal).
      if (previous.current?.isConnected) previous.current.focus();
    };
  }, []);

  useEffect(() => {
    listRef.current
      ?.querySelector<HTMLElement>(`[data-index="${String(current)}"]`)
      ?.scrollIntoView({ block: 'nearest' });
  }, [current]);

  function close() {
    setOpen(false);
  }

  function run(command: Command | undefined) {
    if (!command) return;
    // O comando decide o foco (nova aba foca o terminal), não o elemento anterior.
    previous.current = null;
    close();
    command.run();
  }

  function onKeyDown(e: React.KeyboardEvent) {
    const n = shown.length;
    if (e.key === 'ArrowDown' && n > 0) setSelected((current + 1) % n);
    else if (e.key === 'ArrowUp' && n > 0) setSelected((current - 1 + n) % n);
    else if (e.key === 'PageDown' && n > 0) setSelected(Math.min(n - 1, current + 8));
    else if (e.key === 'PageUp' && n > 0) setSelected(Math.max(0, current - 8));
    else if (e.key === 'Enter') run(shown[current]);
    else if (e.key === 'Escape') close();
    else if (e.key === 'Tab') {
      // Foco preso no campo: a lista é navegada pelas setas.
    } else return;
    e.preventDefault();
  }

  return createPortal(
    <div
      className={s.backdrop}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) close();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t.palette.label}
        data-palette
        className={`${s.palette} raised glass-edge`}
        onKeyDown={onKeyDown}
      >
        <div className={s.field}>
          <SearchIcon className={s.fieldIcon} />
          <input
            ref={inputRef}
            className={s.input}
            role="combobox"
            aria-expanded="true"
            aria-controls={listId}
            aria-activedescendant={shown.length ? `${listId}-${String(current)}` : undefined}
            aria-label={t.palette.label}
            placeholder={t.palette.placeholder}
            spellCheck={false}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelected(0);
            }}
          />
          <kbd className={s.kbd}>Esc</kbd>
        </div>
        {shown.length === 0 ? (
          <p className={s.empty}>{t.palette.empty}</p>
        ) : (
          <ul
            ref={listRef}
            id={listId}
            role="listbox"
            className={s.list}
            aria-label={t.palette.label}
          >
            {shown.map((command, i) => (
              <li
                key={command.id}
                id={`${listId}-${String(i)}`}
                role="option"
                aria-selected={i === current}
                data-index={i}
                className={s.item}
                onMouseMove={() => {
                  if (i !== current) setSelected(i);
                }}
                onMouseDown={(e) => {
                  e.preventDefault();
                  run(command);
                }}
              >
                <span className={s.itemIcon}>{(command as Entry).icon}</span>
                <span className={s.itemLabel}>{command.label}</span>
                {command.hint && <span className={s.itemHint}>{command.hint}</span>}
              </li>
            ))}
          </ul>
        )}
        <div className={s.footer} aria-hidden>
          <span>
            <kbd className={s.kbd}>↑</kbd>
            <kbd className={s.kbd}>↓</kbd> {t.palette.navigate}
          </span>
          <span>
            <kbd className={s.kbd}>Enter</kbd> {t.palette.run}
          </span>
          <span className={s.footerCount}>{t.palette.results(matches.length)}</span>
        </div>
      </div>
    </div>,
    document.body,
  );
}
