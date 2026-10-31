// Importar/Exportar: Tabby, planilha CSV e backup cifrado (.otbackup). Só funciona com o cofre
// aberto; os resumos mostram contagens, nunca nomes, endereços ou senhas.
import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { TextField } from '../../ui/TextField';
import { toastError, toastInfo } from '../../ui/Toast';
import s from './settings.module.css';

type Busy = 'tabby' | 'csv' | 'backup' | null;

export function TransferSection() {
  const unlocked = useVault((v) => v.status?.state === 'unlocked');
  const reload = useHosts((h) => h.reload);
  const [busy, setBusy] = useState<Busy>(null);
  const [dialog, setDialog] = useState<'export' | 'import' | null>(null);

  if (!unlocked) return <p className={s.notice}>{t.settings.vaultLocked}</p>;

  async function importTabby() {
    setBusy('tabby');
    const res = await window.openterminal.hosts.importTabby();
    setBusy(null);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    toastInfo(t.hosts.importTabbyDone(res.value));
    await reload();
  }

  async function importCsv() {
    setBusy('csv');
    const res = await window.openterminal.hosts.importCsv();
    setBusy(null);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    toastInfo(t.transfer.csvDone(res.value));
    await reload();
  }

  return (
    <>
      <div className={s.group}>
        <h3 className={s.groupTitle}>{t.transfer.tabbyTitle}</h3>
        <div className={s.actions}>
          <Button disabled={busy !== null} onClick={() => void importTabby()}>
            {t.hosts.importTabby}
          </Button>
        </div>
        <p className={s.hint}>{t.transfer.tabbyHint}</p>
      </div>
      <div className={s.group}>
        <h3 className={s.groupTitle}>{t.transfer.csvTitle}</h3>
        <div className={s.actions}>
          <Button disabled={busy !== null} onClick={() => void importCsv()}>
            {t.transfer.csvImport}
          </Button>
        </div>
        <p className={s.hint}>{t.transfer.csvHint}</p>
      </div>
      <div className={s.group}>
        <h3 className={s.groupTitle}>{t.transfer.backupTitle}</h3>
        <div className={s.actions}>
          <Button
            disabled={busy !== null}
            onClick={() => {
              setDialog('export');
            }}
          >
            {t.transfer.backupExport}
          </Button>
          <Button
            disabled={busy !== null}
            onClick={() => {
              setDialog('import');
            }}
          >
            {t.transfer.backupImport}
          </Button>
        </div>
        <p className={s.hint}>{t.transfer.backupHint}</p>
      </div>
      {dialog && (
        <BackupPasswordDialog
          mode={dialog}
          onClose={() => {
            setDialog(null);
          }}
          onDone={async () => {
            setDialog(null);
            await reload();
          }}
        />
      )}
    </>
  );
}

function BackupPasswordDialog({
  mode,
  onClose,
  onDone,
}: {
  mode: 'export' | 'import';
  onClose: () => void;
  onDone: () => Promise<void>;
}) {
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const exporting = mode === 'export';
  const tooShort = Array.from(password).length < 8;
  const mismatch = exporting && confirmation.length > 0 && confirmation !== password;
  const valid = exporting ? !tooShort && password === confirmation : password.length > 0;

  async function submit() {
    if (!valid) return;
    setBusy(true);
    setError(null);
    if (exporting) {
      const res = await window.openterminal.hosts.exportBackup(password);
      setBusy(false);
      if (!res.ok) {
        setError(res.error.message);
        return;
      }
      if (res.value) toastInfo(t.transfer.backupSaved(res.value));
    } else {
      const res = await window.openterminal.hosts.importBackup(password);
      setBusy(false);
      if (!res.ok) {
        setError(res.error.message);
        return;
      }
      toastInfo(t.transfer.backupDone(res.value));
    }
    await onDone();
  }

  return (
    <Dialog
      title={exporting ? t.transfer.backupExport : t.transfer.backupImport}
      onClose={onClose}
      onSubmit={() => void submit()}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button type="submit" variant="primary" disabled={busy || !valid}>
            {exporting ? t.transfer.chooseWhere : t.transfer.chooseFile}
          </Button>
        </>
      }
    >
      <p className={s.hint}>{exporting ? t.transfer.exportWarning : t.transfer.importInfo}</p>
      <TextField
        label={t.transfer.backupPassword}
        secret
        autoFocus
        value={password}
        hint={exporting ? t.vault.minLength : undefined}
        error={exporting && password.length > 0 && tooShort ? t.vault.minLength : null}
        onChange={(e) => {
          setPassword(e.target.value);
        }}
      />
      {exporting && (
        <TextField
          label={t.transfer.backupConfirmation}
          secret
          value={confirmation}
          error={mismatch ? t.vault.mismatch : null}
          onChange={(e) => {
            setConfirmation(e.target.value);
          }}
        />
      )}
      {error && (
        <p role="alert" style={{ margin: 0, color: 'var(--danger)' }}>
          {error}
        </p>
      )}
    </Dialog>
  );
}
