import { useEffect, useRef, useState } from 'react';
import type { HostView } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { Button } from '../../ui/Button';
import { ChevronLeftIcon, FolderIcon, PlusIcon, ServerIcon } from '../../ui/icons';
import { toggleSidebar } from '../terminal/commands';
import { Menu } from '../../ui/Menu';
import { GroupNameDialog } from './GroupDialogs';
import { HostForm } from './HostForm';
import { HostList, mutateHosts } from './HostList';
import { SearchBox } from './SearchBox';
import s from './hosts.module.css';

type Open = { kind: 'form'; host: HostView | null } | { kind: 'group' } | null;

export function HostSidebar() {
  const { reload, clear } = useHosts();
  const [open, setOpen] = useState<Open>(null);
  const [query, setQuery] = useState('');
  const [addMenu, setAddMenu] = useState<HTMLElement | null>(null);
  const listRef = useRef<HTMLUListElement>(null);

  // Montado só com o cofre desbloqueado; ao bloquear, os dados saem da interface.
  useEffect(() => {
    void reload();
    return clear;
  }, [reload, clear]);

  return (
    <div className={s.sidebar}>
      <div className={s.header}>
        <h2 className={s.title}>{t.hosts.title}</h2>
        <Button
          variant="ghost"
          iconOnly
          aria-label={t.sidebar.collapse}
          title={t.sidebar.collapse}
          onClick={toggleSidebar}
        >
          <ChevronLeftIcon />
        </Button>
        <Button
          variant="ghost"
          iconOnly
          aria-label={t.hosts.addMenu}
          title={t.hosts.addMenu}
          aria-haspopup="menu"
          aria-expanded={addMenu !== null}
          onClick={(e) => {
            const el = e.currentTarget;
            setAddMenu((cur) => (cur ? null : el));
          }}
        >
          <PlusIcon />
        </Button>
      </div>
      {addMenu && (
        <Menu
          anchor={addMenu}
          items={[
            {
              label: t.hosts.add,
              icon: <ServerIcon />,
              onSelect: () => {
                setOpen({ kind: 'form', host: null });
              },
            },
            {
              label: t.groups.add,
              icon: <FolderIcon />,
              onSelect: () => {
                setOpen({ kind: 'group' });
              },
            },
          ]}
          onClose={() => {
            setAddMenu(null);
          }}
        />
      )}
      <SearchBox
        value={query}
        onChange={setQuery}
        onFocusResults={() => {
          listRef.current?.querySelector<HTMLElement>('[tabindex="0"]')?.focus();
        }}
      />
      <HostList
        query={query}
        listRef={listRef}
        onEdit={(host) => {
          setOpen({ kind: 'form', host });
        }}
        onAdd={() => {
          setOpen({ kind: 'form', host: null });
        }}
      />
      {open?.kind === 'form' && (
        <HostForm
          host={open.host}
          onClose={() => {
            setOpen(null);
          }}
        />
      )}
      {open?.kind === 'group' && (
        <GroupNameDialog
          title={t.groups.add}
          onClose={() => {
            setOpen(null);
          }}
          onSubmit={(name) => mutateHosts(window.openterminal.groups.create(name, null))}
        />
      )}
    </div>
  );
}
