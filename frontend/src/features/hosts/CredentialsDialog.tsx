// Senhas e chaves do cofre (FR-008, FR-009). A interface só vê resumos, nunca segredos.
import { useState } from 'react';
import type { CredentialSummary } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { Button } from '../../ui/Button';
import { ConfirmDialog } from '../../ui/ConfirmDialog';
import { Dialog } from '../../ui/Dialog';
import { TextAreaField, TextField } from '../../ui/TextField';
import { toastError, toastInfo } from '../../ui/Toast';
import { KeyIcon, LockIcon } from '../../ui/icons';
import s from './hosts.module.css';

type Mode = 'password' | 'import' | 'generate';

function CredentialItem({ cred, onRemove }: { cred: CredentialSummary; onRemove: () => void }) {
  async function copy() {
    const res = await window.openterminal.credentials.copyPublicKey(cred.id);
    if (res.ok) toastInfo(t.credentials.copied);
    else toastError(res.error);
  }
  return (
    <li className={s.cred}>
      {cred.kind === 'key' ? <KeyIcon /> : <LockIcon />}
      <div className={s.credText}>
        <span>
          {cred.name}{' '}
          <span className="muted">
            ·{' '}
            {cred.kind === 'key'
              ? (cred.keyType ?? t.credentials.kindKey)
              : t.credentials.kindPassword}
            {cred.origin === 'generated' ? ` · ${t.credentials.generated}` : ''} ·{' '}
            {t.credentials.usedBy(cred.usedByHostCount)}
          </span>
        </span>
        {cred.fingerprint && <span className={s.mono}>{cred.fingerprint}</span>}
      </div>
      {cred.kind === 'key' && (
        <Button variant="ghost" onClick={() => void copy()}>
          {t.credentials.copyPublicKey}
        </Button>
      )}
      <Button
        variant="ghost"
        disabled={cred.usedByHostCount > 0}
        title={cred.usedByHostCount > 0 ? t.credentials.inUse : undefined}
        onClick={onRemove}
      >
        {t.common.remove}
      </Button>
    </li>
  );
}

export function CredentialsDialog({ onClose }: { onClose: () => void }) {
  const { credentials, reload } = useHosts();
  const [mode, setMode] = useState<Mode>('import');
  const [name, setName] = useState('');
  const [secret, setSecret] = useState('');
  const [pem, setPem] = useState('');
  const [filePath, setFilePath] = useState<string | null>(null);
  const [passphrase, setPassphrase] = useState('');
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState<CredentialSummary | null>(null);

  function reset() {
    setName('');
    setSecret('');
    setPem('');
    setFilePath(null);
    setPassphrase('');
  }

  async function pickFile() {
    const res = await window.openterminal.credentials.pickKeyFile();
    if (!res.ok) toastError(res.error);
    else if (res.value) {
      setFilePath(res.value.filePath);
      setPem('');
      if (!name) setName(res.value.filePath.split(/[\\/]/).pop() ?? '');
    }
  }

  async function submit() {
    const api = window.openterminal.credentials;
    setBusy(true);
    const res =
      mode === 'password'
        ? await api.createPassword(name, secret)
        : mode === 'generate'
          ? await api.generateEd25519(name, passphrase || null)
          : await api.importKey(name, filePath ? { filePath } : { pem }, passphrase || null);
    setBusy(false);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    if (mode === 'generate') {
      const copied = await api.copyPublicKey(res.value.id);
      if (copied.ok) toastInfo(t.credentials.copied);
    }
    reset();
    await reload();
  }

  async function remove(cred: CredentialSummary) {
    setRemoving(null);
    const res = await window.openterminal.credentials.remove(cred.id);
    if (!res.ok) toastError(res.error);
    await reload();
  }

  const canSubmit =
    name.trim() !== '' &&
    (mode === 'password'
      ? secret !== ''
      : mode === 'import'
        ? Boolean(filePath) || pem !== ''
        : true);

  if (removing) {
    return (
      <ConfirmDialog
        title={t.common.remove}
        message={t.credentials.removeConfirm(removing.name)}
        confirmLabel={t.common.remove}
        danger
        onCancel={() => {
          setRemoving(null);
        }}
        onConfirm={() => void remove(removing)}
      />
    );
  }

  return (
    <Dialog
      title={t.credentials.title}
      onClose={onClose}
      width={620}
      actions={<Button onClick={onClose}>{t.common.close}</Button>}
    >
      {credentials.length === 0 ? (
        <p className="muted" style={{ margin: 0 }}>
          {t.credentials.empty}
        </p>
      ) : (
        <ul className={s.credList}>
          {credentials.map((c) => (
            <CredentialItem
              key={c.id}
              cred={c}
              onRemove={() => {
                setRemoving(c);
              }}
            />
          ))}
        </ul>
      )}

      <div className={s.tabs} role="tablist">
        {(
          [
            ['import', t.credentials.importKey],
            ['generate', t.credentials.generate],
            ['password', t.credentials.addPassword],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            type="button"
            role="tab"
            aria-selected={mode === value}
            className={s.tabButton}
            onClick={() => {
              setMode(value);
              reset();
            }}
          >
            {label}
          </button>
        ))}
      </div>

      <form
        autoComplete="off"
        className={s.form}
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) void submit();
        }}
      >
        <TextField
          label={t.credentials.name}
          maxLength={100}
          value={name}
          onChange={(e) => {
            setName(e.target.value);
          }}
        />
        {mode === 'password' && (
          <TextField
            label={t.credentials.password}
            secret
            value={secret}
            onChange={(e) => {
              setSecret(e.target.value);
            }}
          />
        )}
        {mode === 'import' && (
          <>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <Button onClick={() => void pickFile()}>{t.credentials.importFromFile}</Button>
              {filePath && <span className={s.mono}>{filePath}</span>}
            </div>
            {!filePath && (
              <TextAreaField
                label={t.credentials.pasteKey}
                value={pem}
                onChange={(e) => {
                  setPem(e.target.value);
                }}
              />
            )}
          </>
        )}
        {mode !== 'password' && (
          <TextField
            label={`${t.credentials.passphrase} (${t.common.optional})`}
            secret
            value={passphrase}
            onChange={(e) => {
              setPassphrase(e.target.value);
            }}
          />
        )}
        <Button
          type="submit"
          variant="primary"
          disabled={busy || !canSubmit}
          style={{ alignSelf: 'flex-end' }}
        >
          {mode === 'generate' ? t.credentials.generate : t.common.save}
        </Button>
      </form>
    </Dialog>
  );
}
