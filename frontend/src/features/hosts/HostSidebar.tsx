import { useEffect, useRef, useState } from 'react';
import type { HostView } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { Button } from '../../ui/Button';
import { FolderIcon, KeyIcon, PlusIcon } from '../../ui/icons';
import { CredentialsDialog } from './CredentialsDialog';
import { GroupNameDialog } from './GroupDialogs';
import { HostForm } from './HostForm';
import { HostList, mutateHosts } from './HostList';
import { SearchBox } from './SearchBox';
import s from './hosts.module.css';

type Open =
  { kind: 'form'; host: HostView | null } | { kind: 'credentials' } | { kind: 'group' } | null;

export function HostSidebar() {
  const { reload, clear } = useHosts();
  const [open, setOpen] = useState<Open>(null);
  const [query, setQuery] = useState('');
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
          aria-label={t.hosts.manageCredentials}
          title={t.hosts.manageCredentials}
          onClick={() => {
            setOpen({ kind: 'credentials' });
          }}
        >
          <KeyIcon />
        </Button>
        <Button
          variant="ghost"
          iconOnly
          aria-label={t.groups.add}
          title={t.groups.add}
          onClick={() => {
            setOpen({ kind: 'group' });
          }}
        >
          <FolderIcon />
        </Button>
        <Button
          variant="ghost"
          iconOnly
          aria-label={t.hosts.add}
          title={t.hosts.add}
          onClick={() => {
            setOpen({ kind: 'form', host: null });
          }}
        >
          <PlusIcon />
        </Button>
      </div>
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
          onManageCredentials={() => {
            setOpen({ kind: 'credentials' });
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
      {open?.kind === 'credentials' && (
        <CredentialsDialog
          onClose={() => {
            setOpen(null);
          }}
        />
      )}
    </div>
  );
}
