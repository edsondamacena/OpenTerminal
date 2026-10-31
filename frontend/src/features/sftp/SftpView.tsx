// Navegador de arquivos remoto (FR-019–FR-022), em painel sólido (FR-032).
import { useEffect, useState } from 'react';
import type { Result } from '@shared/ipc/result';
import type { RemoteEntry } from '@shared/model/types';
import { fmtBytes, fmtDate, t } from '../../i18n/pt-BR';
import type { Tab } from '../../stores/sessions';
import { useTransfers } from '../../stores/transfers';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { ConfirmDialog } from '../../ui/ConfirmDialog';
import {
  ChevronIcon,
  DownloadIcon,
  FileIcon,
  FolderIcon,
  PlusIcon,
  RefreshIcon,
  UploadIcon,
} from '../../ui/icons';
import { Menu, type MenuItem } from '../../ui/Menu';
import type { PopoverAnchor } from '../../ui/Popover';
import { NameDialog } from '../../ui/NameDialog';
import { toastError } from '../../ui/Toast';
import { focusUnlock } from '../vault/UnlockScreen';
import { homeOf } from './openSftp';
import { TransferPanel } from './TransferPanel';
import s from './sftp.module.css';

const parentOf = (path: string) => {
  const trimmed = path.replace(/\/+$/, '');
  const i = trimmed.lastIndexOf('/');
  return i <= 0 ? '/' : trimmed.slice(0, i);
};
const join = (dir: string, name: string) => (dir.endsWith('/') ? dir + name : `${dir}/${name}`);

function validName(name: string): string | null {
  return name.includes('/') || name === '.' || name === '..' ? t.sftp.invalidName : null;
}

/** Segmentos clicáveis do caminho: raiz e cada pasta. */
function crumbs(path: string): { label: string; path: string }[] {
  const parts = path.split('/').filter(Boolean);
  return [
    { label: '/', path: '/' },
    ...parts.map((label, i) => ({ label, path: `/${parts.slice(0, i + 1).join('/')}` })),
  ];
}

type Pending =
  | { kind: 'mkdir' }
  | { kind: 'rename'; entry: RemoteEntry }
  | { kind: 'remove'; entries: RemoteEntry[] };

/** Mostra o erro, se houver, e diz se a chamada deu certo. */
async function check(call: Promise<Result<unknown>>): Promise<boolean> {
  const res = await call;
  if (!res.ok) toastError(res.error);
  return res.ok;
}

export interface SftpViewProps {
  tab: Tab;
  active: boolean;
}

export function SftpView({ tab, active }: SftpViewProps) {
  const api = window.openterminal.sftp;
  const sessionId = tab.id;
  const [path, setPath] = useState(() => homeOf(sessionId));
  const [shown, setShown] = useState(path);
  const [reloads, setReloads] = useState(0);
  const [entries, setEntries] = useState<RemoteEntry[]>([]);
  // A lista só é exibida como vazia depois da primeira resposta do servidor.
  const [loaded, setLoaded] = useState(false);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [anchor, setAnchor] = useState<string | null>(null);
  const [menu, setMenu] = useState<{ anchor: PopoverAnchor; items: MenuItem[] } | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  const connected = tab.state === 'connected';

  // Envio concluído para a pasta aberta também recarrega a lista.
  const uploadsDone = useTransfers(
    (st) =>
      st.transfers.filter(
        (x) =>
          x.sessionId === sessionId &&
          x.direction === 'upload' &&
          x.state === 'completed' &&
          parentOf(x.remotePath) === path,
      ).length,
  );

  // `path` é a pasta pedida; `shown` é a última listada com sucesso.
  useEffect(() => {
    if (!connected) return;
    let live = true;
    void window.openterminal.sftp.list(sessionId, path).then((res) => {
      if (!live) return;
      setLoaded(true);
      if (!res.ok) {
        toastError(res.error);
        setPath(shown);
        return;
      }
      setShown(path);
      setEntries(res.value);
      setSelected(new Set());
      setAnchor(null);
    });
    return () => {
      live = false;
    };
    // `shown` só serve para voltar após erro; não dispara nova listagem.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connected, sessionId, path, reloads, uploadsDone]);

  const load = (target: string) => {
    if (target === path) refresh();
    else setPath(target);
  };
  const refresh = () => {
    setReloads((n) => n + 1);
  };

  const selectedEntries = entries.filter((e) => selected.has(e.path));
  const selectedFiles = selectedEntries.filter((e) => e.type === 'file');

  function select(entry: RemoteEntry, e: React.MouseEvent) {
    if (e.shiftKey && anchor) {
      const a = entries.findIndex((x) => x.path === anchor);
      const b = entries.findIndex((x) => x.path === entry.path);
      const [from, to] = a < b ? [a, b] : [b, a];
      setSelected(new Set(entries.slice(from, to + 1).map((x) => x.path)));
      return;
    }
    setAnchor(entry.path);
    if (e.ctrlKey) {
      const next = new Set(selected);
      if (next.has(entry.path)) next.delete(entry.path);
      else next.add(entry.path);
      setSelected(next);
    } else {
      setSelected(new Set([entry.path]));
    }
  }

  function open(entry: RemoteEntry) {
    if (entry.type === 'dir' || entry.type === 'link') load(entry.path);
    else void download([entry]);
  }

  async function download(list: RemoteEntry[]) {
    const files = list.filter((e) => e.type === 'file');
    if (files.length)
      await check(
        api.download(
          sessionId,
          files.map((e) => e.path),
        ),
      );
  }

  async function upload() {
    await check(api.upload(sessionId, path));
  }

  function rowMenu(entry: RemoteEntry): MenuItem[] {
    const targets = selected.has(entry.path) ? selectedEntries : [entry];
    return [
      {
        label: t.sftp.download,
        icon: <DownloadIcon />,
        disabled: !targets.some((e) => e.type === 'file'),
        onSelect: () => void download(targets),
      },
      {
        label: t.sftp.rename,
        disabled: targets.length !== 1,
        onSelect: () => {
          setPending({ kind: 'rename', entry });
        },
      },
      {
        label: t.sftp.remove,
        danger: true,
        onSelect: () => {
          setPending({ kind: 'remove', entries: targets });
        },
      },
    ];
  }

  function backgroundMenu(): MenuItem[] {
    return [
      {
        label: t.sftp.newFolder,
        onSelect: () => {
          setPending({ kind: 'mkdir' });
        },
      },
      { label: t.sftp.upload, onSelect: () => void upload() },
      {
        label: t.sftp.refresh,
        onSelect: () => {
          load(path);
        },
      },
    ];
  }

  async function reconnect() {
    if (useVault.getState().status?.state !== 'unlocked') {
      toastError(t.terminal.unlockToReconnect);
      focusUnlock();
      return;
    }
    await check(window.openterminal.sessions.reconnect(sessionId));
  }

  function renderPending(p: Pending) {
    const close = () => {
      setPending(null);
    };
    switch (p.kind) {
      case 'mkdir':
        return (
          <NameDialog
            title={t.sftp.newFolder}
            label={t.sftp.folderName}
            validate={validName}
            onClose={close}
            onSubmit={async (name) => {
              const ok = await check(api.mkdir(sessionId, join(path, name)));
              if (ok) refresh();
              return ok;
            }}
          />
        );
      case 'rename':
        return (
          <NameDialog
            title={t.sftp.renameTitle}
            label={t.sftp.newName}
            initial={p.entry.name}
            selectStem={p.entry.type === 'file'}
            validate={validName}
            onClose={close}
            onSubmit={async (name) => {
              if (name === p.entry.name) return true;
              const ok = await check(
                api.rename(sessionId, p.entry.path, join(parentOf(p.entry.path), name)),
              );
              if (ok) refresh();
              return ok;
            }}
          />
        );
      case 'remove':
        return (
          <ConfirmDialog
            title={t.sftp.removeTitle}
            message={t.sftp.removeConfirm(p.entries.length, p.entries[0]?.name ?? '')}
            confirmLabel={t.sftp.remove}
            danger
            onCancel={close}
            onConfirm={() => {
              close();
              void (async () => {
                for (const entry of p.entries) {
                  if (!(await check(api.remove(sessionId, entry.path, true)))) break;
                }
                refresh();
              })();
            }}
          />
        );
    }
  }

  return (
    <div
      className={`${s.view} solid ${active ? '' : s.hidden}`}
      role="tabpanel"
      aria-label={tab.title}
      data-session-id={sessionId}
    >
      {tab.state === 'disconnected' && (
        <div className={s.banner} role="status">
          <span className={s.bannerText}>
            {t.terminal.disconnected}
            {tab.reason ? ` — ${tab.reason}` : ''}
          </span>
          <Button variant="primary" onClick={() => void reconnect()}>
            <RefreshIcon />
            {t.terminal.reconnect}
          </Button>
        </div>
      )}

      <div className={s.toolbar} role="toolbar" aria-label={t.sftp.files}>
        <Button
          iconOnly
          variant="ghost"
          aria-label={t.sftp.up}
          title={t.sftp.up}
          disabled={!connected || path === '/'}
          onClick={() => {
            load(parentOf(path));
          }}
        >
          <ChevronIcon className={s.upIcon} />
        </Button>
        <nav className={s.crumbs} aria-label={t.sftp.location}>
          {crumbs(path).map((c, i, all) => (
            <span key={c.path} className={s.crumb}>
              <button
                type="button"
                className={s.crumbButton}
                aria-current={i === all.length - 1 ? 'location' : undefined}
                disabled={!connected}
                onClick={() => {
                  load(c.path);
                }}
              >
                {c.label === '/' ? t.sftp.root : c.label}
              </button>
              {i < all.length - 1 && <span className={s.crumbSep}>/</span>}
            </span>
          ))}
        </nav>
        <Button
          iconOnly
          variant="ghost"
          aria-label={t.sftp.refresh}
          title={t.sftp.refresh}
          disabled={!connected}
          onClick={() => {
            load(path);
          }}
        >
          <RefreshIcon />
        </Button>
        <Button
          iconOnly
          variant="ghost"
          aria-label={t.sftp.newFolder}
          title={t.sftp.newFolder}
          disabled={!connected}
          onClick={() => {
            setPending({ kind: 'mkdir' });
          }}
        >
          <PlusIcon />
        </Button>
        <Button disabled={!connected} onClick={() => void upload()}>
          <UploadIcon />
          {t.sftp.upload}
        </Button>
        <Button
          disabled={!connected || selectedFiles.length === 0}
          onClick={() => void download(selectedFiles)}
        >
          <DownloadIcon />
          {t.sftp.download}
        </Button>
      </div>

      <div
        className={s.tableWrap}
        aria-busy={!loaded}
        onContextMenu={(e) => {
          if (e.target !== e.currentTarget || !connected) return;
          e.preventDefault();
          setMenu({ anchor: { x: e.clientX, y: e.clientY }, items: backgroundMenu() });
        }}
      >
        <table className={s.table} aria-label={t.sftp.files} aria-multiselectable="true">
          <thead>
            <tr>
              <th className={s.colName}>{t.sftp.name}</th>
              <th className={s.colSize}>{t.sftp.size}</th>
              <th className={s.colDate}>{t.sftp.modified}</th>
              <th className={s.colType}>{t.sftp.type}</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry) => (
              <tr
                key={entry.path}
                aria-selected={selected.has(entry.path)}
                tabIndex={0}
                className={s.row}
                onClick={(e) => {
                  select(entry, e);
                }}
                onDoubleClick={() => {
                  open(entry);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') open(entry);
                  else if (e.key === 'Delete')
                    setPending({
                      kind: 'remove',
                      entries: selected.has(entry.path) ? selectedEntries : [entry],
                    });
                  else if (e.key === 'F2') setPending({ kind: 'rename', entry });
                  else if (e.key === 'Backspace') load(parentOf(path));
                  else return;
                  e.preventDefault();
                }}
                onContextMenu={(e) => {
                  e.preventDefault();
                  if (!selected.has(entry.path)) setSelected(new Set([entry.path]));
                  setMenu({ anchor: { x: e.clientX, y: e.clientY }, items: rowMenu(entry) });
                }}
              >
                <td className={s.colName}>
                  {entry.type === 'dir' ? (
                    <FolderIcon className={s.icon} />
                  ) : (
                    <FileIcon className={s.icon} />
                  )}
                  <span className={s.name}>{entry.name}</span>
                </td>
                <td className={s.colSize}>{entry.type === 'dir' ? '' : fmtBytes(entry.size)}</td>
                <td className={s.colDate}>{fmtDate(entry.modifiedAt)}</td>
                <td className={s.colType}>{t.sftp.types[entry.type]}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {connected && loaded && entries.length === 0 && <p className={s.empty}>{t.sftp.empty}</p>}
      </div>

      <TransferPanel sessionId={sessionId} />

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
    </div>
  );
}
