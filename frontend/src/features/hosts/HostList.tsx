// Árvore de grupos e hosts (US3) e resultados de busca planos com o caminho do grupo.
import { useLayoutEffect, useMemo, useState, type RefObject } from 'react';
import type { Result } from '@shared/ipc/result';
import type { Group, HostView } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useMobile } from '../../app/mobile';
import { useHosts } from '../../stores/hosts';
import { useUi } from '../../stores/ui';
import { ConfirmDialog } from '../../ui/ConfirmDialog';
import { Menu, type MenuItem } from '../../ui/Menu';
import type { PopoverAnchor } from '../../ui/Popover';
import { useSlidingIndicator } from '../../ui/motion';
import { DeviceLogo, hostDeviceIcon } from '../../ui/deviceIcons';
import { Button } from '../../ui/Button';
import { ChevronIcon, FolderIcon, FolderOpenIcon, PlusIcon } from '../../ui/icons';
import { toastError } from '../../ui/Toast';
import { ForwardRulesPanel } from '../forwarding/ForwardRulesPanel';
import { openSftp } from '../sftp/openSftp';
import { connectHost } from './connect';
import { GroupNameDialog, MoveDialog } from './GroupDialogs';
import { buildIndex, filterHosts } from './search';
import { buildRows, groupPaths, rowIndent, type TreeRow } from './tree';
import s from './hosts.module.css';

const HOST_MIME = 'application/x-openterminal-host';
const GROUP_MIME = 'application/x-openterminal-group';
const VIRTUALIZE_AFTER = 200;
const HOST_ROW = 38;
const GROUP_ROW = 32;
const OVERSCAN_PX = 400;

/** Grupos recolhidos ficam nas preferências (FR-167): a árvore muda antes de salvar. */
function setCollapsedGroups(collapsedGroups: string[]): void {
  const ui = useUi.getState();
  if (!ui.settings) return;
  ui.setSettings({ ...ui.settings, collapsedGroups });
  void window.openterminal.app.updateUiSettings({ collapsedGroups }).then((res) => {
    if (!res.ok) toastError(res.error);
  });
}

/** Executa uma mudança na ponte; em sucesso recarrega a lista, em erro mostra aviso. */
export async function mutateHosts(call: Promise<Result<unknown>>): Promise<boolean> {
  const res = await call;
  if (!res.ok) {
    toastError(res.error);
    return false;
  }
  await useHosts.getState().reload();
  return true;
}

type Pending =
  | { kind: 'createGroup'; parentId: string | null }
  | { kind: 'renameGroup'; group: Group }
  | { kind: 'removeGroup'; group: Group }
  | { kind: 'moveGroup'; group: Group }
  | { kind: 'removeHost'; host: HostView }
  | { kind: 'moveHost'; host: HostView }
  | { kind: 'forwards'; host: HostView };

const rowHeight = (row: TreeRow) => (row.kind === 'host' ? HOST_ROW : GROUP_ROW);
const rowKey = (row: TreeRow) => (row.kind === 'host' ? row.host.id : `g:${row.group.id}`);

/** Janela de linhas visíveis quando a lista passa de VIRTUALIZE_AFTER itens. */
function useVirtualWindow(rows: TreeRow[], listRef: RefObject<HTMLUListElement | null>) {
  const enabled = rows.length > VIRTUALIZE_AFTER;
  const [view, setView] = useState({ top: 0, height: 800 });
  const offsets = useMemo(() => {
    const out = [0];
    for (const row of rows) out.push((out[out.length - 1] ?? 0) + rowHeight(row));
    return out;
  }, [rows]);

  useLayoutEffect(() => {
    const el = listRef.current;
    if (!el || !enabled) return;
    const update = () => {
      setView({ top: el.scrollTop, height: el.clientHeight });
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    el.addEventListener('scroll', update, { passive: true });
    return () => {
      observer.disconnect();
      el.removeEventListener('scroll', update);
    };
  }, [enabled, listRef]);

  const total = offsets[offsets.length - 1] ?? 0;
  if (!enabled) return { start: 0, end: rows.length, padTop: 0, padBottom: 0 };
  const from = view.top - OVERSCAN_PX;
  const to = view.top + view.height + OVERSCAN_PX;
  let start = 0;
  while (start < rows.length && (offsets[start + 1] ?? 0) < from) start++;
  let end = start;
  while (end < rows.length && (offsets[end] ?? 0) < to) end++;
  return {
    start,
    end,
    padTop: offsets[start] ?? 0,
    padBottom: total - (offsets[end] ?? total),
  };
}

function moveFocus(from: HTMLElement, delta: 1 | -1) {
  const target = delta === 1 ? from.nextElementSibling : from.previousElementSibling;
  if (target instanceof HTMLElement && target.tabIndex === 0) target.focus();
}

export interface HostListProps {
  query: string;
  listRef: RefObject<HTMLUListElement | null>;
  onEdit: (host: HostView) => void;
  onAdd: () => void;
}

export function HostList({ query, listRef, onEdit, onAdd }: HostListProps) {
  const { hosts, groups, loaded } = useHosts();
  const [selected, setSelected] = useState<string | null>(null);
  const mobile = useMobile();
  const collapsedIds = useUi((u) => u.settings?.collapsedGroups);
  const collapsed = useMemo<ReadonlySet<string>>(() => new Set(collapsedIds), [collapsedIds]);
  const [menu, setMenu] = useState<{ anchor: PopoverAnchor; items: MenuItem[] } | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  const [dropTarget, setDropTarget] = useState<string | null>(null);

  const index = useMemo(() => buildIndex(hosts), [hosts]);
  const paths = useMemo(() => groupPaths(groups), [groups]);
  const searching = query.trim() !== '';
  const rows = useMemo<TreeRow[]>(
    () =>
      searching
        ? filterHosts(index, query)
            .sort((a, b) => a.label.localeCompare(b.label, 'pt-BR'))
            .map((host) => ({ kind: 'host', host, depth: 0 }))
        : buildRows(groups, hosts, collapsed),
    [searching, index, query, groups, hosts, collapsed],
  );
  const win = useVirtualWindow(rows, listRef);
  // Linhas acima mudam de altura ao recolher grupos: a lista entra na chave.
  const indicator = useSlidingIndicator(listRef, `${selected ?? ''}|${rows.length}|${query}`, 'y');

  const toggle = (groupId: string, expand?: boolean) => {
    const open = expand ?? collapsed.has(groupId);
    if (open !== collapsed.has(groupId)) return;
    setCollapsedGroups(
      open ? [...collapsed].filter((id) => id !== groupId) : [...collapsed, groupId],
    );
  };

  function hostMenu(host: HostView): MenuItem[] {
    return [
      { label: t.hosts.connect, onSelect: () => void connectHost(host) },
      { label: t.sftp.open, onSelect: () => void openSftp(host) },
      {
        label: t.forwarding.open,
        onSelect: () => {
          setPending({ kind: 'forwards', host });
        },
      },
      {
        label: t.common.edit,
        onSelect: () => {
          onEdit(host);
        },
      },
      {
        label: t.hosts.duplicate,
        onSelect: () => void mutateHosts(window.openterminal.hosts.duplicate(host.id)),
      },
      {
        label: t.hosts.moveTo,
        onSelect: () => {
          setPending({ kind: 'moveHost', host });
        },
      },
      {
        label: t.common.remove,
        danger: true,
        onSelect: () => {
          setPending({ kind: 'removeHost', host });
        },
      },
    ];
  }

  function groupMenu(group: Group): MenuItem[] {
    return [
      {
        label: t.groups.addSub,
        onSelect: () => {
          setPending({ kind: 'createGroup', parentId: group.id });
        },
      },
      {
        label: t.common.rename,
        onSelect: () => {
          setPending({ kind: 'renameGroup', group });
        },
      },
      {
        label: t.hosts.moveTo,
        onSelect: () => {
          setPending({ kind: 'moveGroup', group });
        },
      },
      {
        label: t.common.remove,
        danger: true,
        onSelect: () => {
          setPending({ kind: 'removeGroup', group });
        },
      },
    ];
  }

  function dropInto(e: React.DragEvent, groupId: string | null) {
    setDropTarget(null);
    const hostId = e.dataTransfer.getData(HOST_MIME);
    const movingGroup = e.dataTransfer.getData(GROUP_MIME);
    if (hostId) {
      e.preventDefault();
      void mutateHosts(window.openterminal.hosts.move(hostId, groupId));
    } else if (movingGroup && movingGroup !== groupId) {
      e.preventDefault();
      void mutateHosts(window.openterminal.groups.move(movingGroup, groupId));
    }
  }

  const accepts = (e: React.DragEvent) =>
    e.dataTransfer.types.includes(HOST_MIME) || e.dataTransfer.types.includes(GROUP_MIME);

  function renderRow(row: TreeRow, position: number) {
    const indent = { paddingLeft: rowIndent(row) };
    if (row.kind === 'group') {
      const { group, expanded } = row;
      const key = rowKey(row);
      return (
        <li
          key={key}
          role="treeitem"
          aria-level={row.depth + 1}
          aria-expanded={expanded}
          aria-selected={selected === key}
          aria-posinset={position + 1}
          aria-setsize={rows.length}
          tabIndex={0}
          draggable
          className={`${s.item} ${s.groupItem} ${dropTarget === group.id ? s.dropInto : ''}`}
          style={indent}
          onClick={() => {
            setSelected(key);
            toggle(group.id);
          }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowRight') toggle(group.id, true);
            else if (e.key === 'ArrowLeft') toggle(group.id, false);
            else if (e.key === 'ArrowDown') moveFocus(e.currentTarget, 1);
            else if (e.key === 'ArrowUp') moveFocus(e.currentTarget, -1);
            else if (e.key === 'Enter') toggle(group.id);
            else if (e.key === 'F2') setPending({ kind: 'renameGroup', group });
            else if (e.key === 'Delete') setPending({ kind: 'removeGroup', group });
            else return;
            e.preventDefault();
          }}
          onContextMenu={(e) => {
            e.preventDefault();
            setSelected(key);
            setMenu({ anchor: { x: e.clientX, y: e.clientY }, items: groupMenu(group) });
          }}
          onDragStart={(e) => {
            e.stopPropagation();
            e.dataTransfer.setData(GROUP_MIME, group.id);
            e.dataTransfer.effectAllowed = 'move';
          }}
          onDragOver={(e) => {
            if (!accepts(e)) return;
            e.preventDefault();
            e.stopPropagation();
            setDropTarget(group.id);
          }}
          onDragLeave={() => {
            setDropTarget((cur) => (cur === group.id ? null : cur));
          }}
          onDrop={(e) => {
            e.stopPropagation();
            dropInto(e, group.id);
          }}
        >
          <ChevronIcon className={`${s.chevron} ${expanded ? s.chevronOpen : ''}`} />
          {expanded ? (
            <FolderOpenIcon className={s.itemIcon} />
          ) : (
            <FolderIcon className={s.itemIcon} />
          )}
          <span className={s.itemLabel}>{group.name}</span>
          {row.hostCount > 0 && <span className={s.count}>{row.hostCount}</span>}
        </li>
      );
    }

    const { host } = row;
    const path = searching && host.groupId ? paths.get(host.groupId) : undefined;
    return (
      <li
        key={host.id}
        role={searching ? 'option' : 'treeitem'}
        aria-level={searching ? undefined : row.depth + 1}
        aria-selected={selected === host.id}
        aria-posinset={position + 1}
        aria-setsize={rows.length}
        tabIndex={0}
        draggable={!searching && !mobile}
        className={`${s.item} ${s.hostItem}`}
        // Host fica mais à esquerda que o grupo: sobra largura para o nome.
        style={{ paddingLeft: Math.max(2, rowIndent(row) - 10) }}
        title={`${host.label}\n${host.username ? `${host.username}@` : ''}${host.address}:${String(host.port)}`}
        onClick={() => {
          setSelected(host.id);
          // No celular um toque conecta (não há duplo clique confortável no toque).
          if (mobile) void connectHost(host);
        }}
        onDoubleClick={() => {
          if (!mobile) void connectHost(host);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void connectHost(host);
          else if (e.key === 'Delete') setPending({ kind: 'removeHost', host });
          else if (e.key === 'ArrowDown') moveFocus(e.currentTarget, 1);
          else if (e.key === 'ArrowUp') moveFocus(e.currentTarget, -1);
          else return;
          e.preventDefault();
        }}
        onContextMenu={(e) => {
          e.preventDefault();
          setSelected(host.id);
          setMenu({ anchor: { x: e.clientX, y: e.clientY }, items: hostMenu(host) });
        }}
        onDragStart={(e) => {
          e.dataTransfer.setData(HOST_MIME, host.id);
          e.dataTransfer.effectAllowed = 'move';
        }}
      >
        {/* Sem o espaço da seta: o ícone fica sob a seta do grupo e sobra largura para o nome. */}
        <DeviceLogo id={hostDeviceIcon(host)} size={15} className={s.itemIcon} />
        <span className={s.itemText}>
          <span className={s.itemLabelRow}>
            <span className={s.itemLabel}>{host.label}</span>
          </span>
          {/* Endereço diferencia hosts com o mesmo nome; na busca, o caminho do grupo. */}
          <span className={s.itemSub}>
            {path ? `${path} · ` : ''}
            {host.address}
            {host.port !== (host.protocol === 'telnet' ? 23 : 22) ? `:${String(host.port)}` : ''}
          </span>
        </span>
      </li>
    );
  }

  function renderPending(p: Pending) {
    const close = () => {
      setPending(null);
    };
    switch (p.kind) {
      case 'createGroup':
        return (
          <GroupNameDialog
            title={p.parentId ? t.groups.addSub : t.groups.add}
            onClose={close}
            onSubmit={async (name) => {
              const done = await mutateHosts(window.openterminal.groups.create(name, p.parentId));
              if (done && p.parentId) toggle(p.parentId, true);
              return done;
            }}
          />
        );
      case 'renameGroup':
        return (
          <GroupNameDialog
            title={t.groups.rename}
            initial={p.group.name}
            onClose={close}
            onSubmit={(name) => mutateHosts(window.openterminal.groups.rename(p.group.id, name))}
          />
        );
      case 'moveGroup':
        return (
          <MoveDialog
            current={p.group.parentId}
            movingGroupId={p.group.id}
            onClose={close}
            onSubmit={(target) => mutateHosts(window.openterminal.groups.move(p.group.id, target))}
          />
        );
      case 'moveHost':
        return (
          <MoveDialog
            current={p.host.groupId}
            onClose={close}
            onSubmit={(target) => mutateHosts(window.openterminal.hosts.move(p.host.id, target))}
          />
        );
      case 'removeGroup':
        return (
          <ConfirmDialog
            title={t.groups.removeTitle}
            message={t.groups.removeConfirm(p.group.name)}
            confirmLabel={t.common.remove}
            danger
            onCancel={close}
            onConfirm={() => {
              close();
              void mutateHosts(window.openterminal.groups.remove(p.group.id));
            }}
          />
        );
      case 'forwards':
        return <ForwardRulesPanel host={p.host} onClose={close} />;
      case 'removeHost':
        return (
          <ConfirmDialog
            title={t.hosts.removeConfirmTitle}
            message={t.hosts.removeConfirm(p.host.label)}
            confirmLabel={t.common.remove}
            danger
            onCancel={close}
            onConfirm={() => {
              close();
              void mutateHosts(window.openterminal.hosts.remove(p.host.id));
            }}
          />
        );
    }
  }

  let content;
  if (loaded && hosts.length === 0 && groups.length === 0) {
    content = (
      <div className={s.empty}>
        <p>{t.hosts.empty}</p>
        <Button onClick={onAdd}>
          <PlusIcon />
          {t.hosts.addFirst}
        </Button>
      </div>
    );
  } else if (searching && rows.length === 0) {
    content = <p className={s.empty}>{t.hosts.noResults}</p>;
  } else {
    content = (
      <ul
        ref={listRef}
        style={indicator ?? undefined}
        data-indicator={indicator ? '' : undefined}
        className={`${s.list} ${dropTarget === 'root' ? s.dropRoot : ''}`}
        role={searching ? 'listbox' : 'tree'}
        aria-label={t.hosts.title}
        onDragOver={(e) => {
          if (searching || !accepts(e)) return;
          e.preventDefault();
          setDropTarget('root');
        }}
        onDragLeave={(e) => {
          if (e.target === e.currentTarget) setDropTarget(null);
        }}
        onDrop={(e) => {
          if (!searching) dropInto(e, null);
        }}
      >
        {win.padTop > 0 && <li aria-hidden style={{ height: win.padTop }} />}
        {rows.slice(win.start, win.end).map((row, i) => renderRow(row, win.start + i))}
        {win.padBottom > 0 && <li aria-hidden style={{ height: win.padBottom }} />}
      </ul>
    );
  }

  return (
    <>
      {content}
      {menu && (
        <Menu
          anchor={menu.anchor}
          items={menu.items}
          onClose={() => {
            setMenu(null);
          }}
        />
      )}
      {pending && renderPending(pending)}
    </>
  );
}
