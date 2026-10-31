// Sincronização do cofre com o Google Drive. A senha mestra só passa pelo Connect (deriva a chave
// da nuvem no Go); a tela nunca a guarda.
import { useEffect, useState } from 'react';
import type { SyncStatus } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { TextField } from '../../ui/TextField';
import { toastError } from '../../ui/Toast';
import s from './settings.module.css';

function formatWhen(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString('pt-BR');
}

export function SyncSection() {
  const unlocked = useVault((v) => v.status?.state === 'unlocked');
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    void window.openterminal.sync.getStatus().then((res) => {
      if (alive && res.ok) setStatus(res.value);
    });
    const off = window.openterminal.sync.onStatus((st) => {
      setStatus(st);
    });
    return () => {
      alive = false;
      off();
    };
  }, []);

  if (!unlocked) return <p className={s.notice}>{t.settings.vaultLocked}</p>;
  if (!status) return null;

  async function run(
    op: () => Promise<
      { ok: true; value: SyncStatus } | { ok: false; error: Parameters<typeof toastError>[0] }
    >,
  ) {
    setBusy(true);
    const res = await op();
    setBusy(false);
    if (res.ok) setStatus(res.value);
    else toastError(res.error);
  }

  const connected = status.connected;
  return (
    <>
      <div className={s.group}>
        <h3 className={s.groupTitle}>{t.sync.googleTitle}</h3>
        <p className={s.hint}>{t.sync.intro}</p>
        {!status.configured && <p className={s.notice}>{t.sync.notConfigured}</p>}
        {connected && (
          <>
            <p>
              {t.sync.connectedAs(status.email)} · {t.sync.states[status.state] ?? status.state}
            </p>
            {status.lastSync && (
              <p className={s.hint}>{t.sync.lastSync(formatWhen(status.lastSync))}</p>
            )}
          </>
        )}
        {status.state === 'error' && status.error && (
          <p role="alert" style={{ margin: 0, color: 'var(--danger)' }}>
            {status.error}
          </p>
        )}
        <div className={s.actions}>
          {status.loggingIn ? (
            <>
              <span className={s.hint}>{t.sync.connecting}</span>
              <Button onClick={() => void window.openterminal.sync.cancelConnect()}>
                {t.sync.cancel}
              </Button>
            </>
          ) : connected ? (
            <>
              <Button
                variant="primary"
                disabled={busy || status.state === 'syncing'}
                onClick={() => void run(() => window.openterminal.sync.syncNow())}
              >
                {t.sync.syncNow}
              </Button>
              <Button
                variant="danger"
                disabled={busy}
                onClick={() => void run(() => window.openterminal.sync.disconnect())}
              >
                {t.sync.disconnect}
              </Button>
            </>
          ) : (
            <Button
              variant="primary"
              disabled={busy || !status.configured}
              onClick={() => {
                setAsking(true);
              }}
            >
              {t.sync.connect}
            </Button>
          )}
        </div>
        <p className={s.hint}>{t.sync.howItMerges}</p>
        <p className={s.hint}>{t.sync.testingNote}</p>
      </div>
      {asking && (
        <PasswordDialog
          onClose={() => {
            setAsking(false);
          }}
          onConnected={setStatus}
        />
      )}
    </>
  );
}

function PasswordDialog({
  onClose,
  onConnected,
}: {
  onClose: () => void;
  onConnected: (st: SyncStatus) => void;
}) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError(null);
    // Volta só depois do login no navegador (ou do cancelamento); até lá o botão mostra a espera.
    const res = await window.openterminal.sync.connect(password);
    setBusy(false);
    if (!res.ok) {
      setError(res.error.message);
      return;
    }
    setPassword('');
    onConnected(res.value);
    onClose();
  }

  return (
    <Dialog
      title={t.sync.passwordTitle}
      onClose={() => {
        if (busy) void window.openterminal.sync.cancelConnect();
        onClose();
      }}
      onSubmit={() => void submit()}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button type="submit" variant="primary" disabled={busy || password.length === 0}>
            {busy ? t.sync.connecting : t.sync.continue}
          </Button>
        </>
      }
    >
      <p className={s.hint}>{t.sync.passwordHint}</p>
      <TextField
        label={t.sync.password}
        secret
        autoFocus
        value={password}
        onChange={(e) => {
          setPassword(e.target.value);
        }}
      />
      {error && (
        <p role="alert" style={{ margin: 0, color: 'var(--danger)' }}>
          {error}
        </p>
      )}
    </Dialog>
  );
}
