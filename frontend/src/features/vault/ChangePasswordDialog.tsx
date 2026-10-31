import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { TextField } from '../../ui/TextField';
import { toastInfo } from '../../ui/Toast';

export function ChangePasswordDialog({ onClose }: { onClose: () => void }) {
  const changePassword = useVault((v) => v.changePassword);
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const tooShort = Array.from(next).length < 8;
  const mismatch = confirmation.length > 0 && confirmation !== next;
  const valid = current.length > 0 && !tooShort && next === confirmation;

  async function submit() {
    if (!valid) return;
    setBusy(true);
    setError(null);
    const res = await changePassword(current, next, confirmation);
    setBusy(false);
    if (!res.ok) {
      setError(res.error.message);
      return;
    }
    toastInfo(t.vault.changePasswordDone);
    onClose();
  }

  return (
    <Dialog
      title={t.vault.changePasswordTitle}
      onClose={onClose}
      onSubmit={() => void submit()}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button type="submit" variant="primary" disabled={busy || !valid}>
            {t.common.save}
          </Button>
        </>
      }
    >
      <TextField
        label={t.vault.currentPassword}
        secret
        autoFocus
        value={current}
        onChange={(e) => {
          setCurrent(e.target.value);
        }}
      />
      <TextField
        label={t.vault.newPassword}
        secret
        value={next}
        hint={t.vault.minLength}
        error={next.length > 0 && tooShort ? t.vault.minLength : null}
        onChange={(e) => {
          setNext(e.target.value);
        }}
      />
      <TextField
        label={t.vault.newConfirmation}
        secret
        value={confirmation}
        error={mismatch ? t.vault.mismatch : null}
        onChange={(e) => {
          setConfirmation(e.target.value);
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
