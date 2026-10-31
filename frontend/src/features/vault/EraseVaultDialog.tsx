// Apagar o cofre exige confirmação dupla (FR-006).
import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { ConfirmDialog } from '../../ui/ConfirmDialog';
import { Dialog } from '../../ui/Dialog';
import { TextField } from '../../ui/TextField';
import { toastError } from '../../ui/Toast';

export function EraseVaultDialog({ onClose }: { onClose: () => void }) {
  const erase = useVault((v) => v.erase);
  const [step, setStep] = useState<'warn' | 'type'>('warn');
  const [phrase, setPhrase] = useState('');
  const [busy, setBusy] = useState(false);

  if (step === 'warn') {
    return (
      <ConfirmDialog
        title={t.vault.eraseTitle}
        message={t.vault.eraseWarning}
        confirmLabel={t.vault.eraseContinue}
        danger
        onCancel={onClose}
        onConfirm={() => {
          setStep('type');
        }}
      />
    );
  }

  async function submit() {
    if (phrase !== t.vault.erasePhrase) return;
    setBusy(true);
    const res = await erase(phrase);
    setBusy(false);
    if (!res.ok) toastError(res.error);
    else onClose();
  }

  return (
    <Dialog
      title={t.vault.eraseTitle}
      onClose={onClose}
      onSubmit={() => void submit()}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button type="submit" variant="danger" disabled={busy || phrase !== t.vault.erasePhrase}>
            {t.vault.eraseAction}
          </Button>
        </>
      }
    >
      <p style={{ margin: 0 }}>{t.vault.eraseWarning}</p>
      <TextField
        label={t.vault.eraseTypePrompt}
        autoFocus
        value={phrase}
        onChange={(e) => {
          setPhrase(e.target.value);
        }}
      />
    </Dialog>
  );
}
