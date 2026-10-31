// Painel de desbloqueio na barra lateral: abas abertas continuam visíveis (FR-003).
import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { TextField } from '../../ui/TextField';
import { LockIcon } from '../../ui/icons';
import { EraseVaultDialog } from './EraseVaultDialog';
import s from './vault.module.css';

export function UnlockPanel() {
  const unlock = useVault((v) => v.unlock);
  // Cofre da 001 ainda não migrado: o mesmo desbloqueio migra (FR-102).
  const migratable = useVault((v) => v.status?.state === 'migratable');
  const text = migratable
    ? {
        title: t.vault.migrateTitle,
        intro: t.vault.migrateIntro,
        action: t.vault.migrateAction,
        busy: t.vault.migrating,
      }
    : {
        title: t.vault.unlockTitle,
        intro: t.vault.unlockIntro,
        action: t.vault.unlockAction,
        busy: t.vault.unlocking,
      };
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [erasing, setErasing] = useState(false);

  async function submit(e: React.SubmitEvent) {
    e.preventDefault();
    if (!password) return;
    setBusy(true);
    setError(null);
    const res = await unlock(password);
    setBusy(false);
    setPassword('');
    if (!res.ok) setError(res.error.message);
  }

  return (
    <>
      <form
        className={s.panel}
        autoComplete="off"
        onSubmit={(e) => void submit(e)}
        aria-label={text.title}
      >
        <h2 className={s.panelTitle}>
          <span className={s.panelIcon}>
            <LockIcon />
          </span>
          {text.title}
        </h2>
        <p className="muted" style={{ margin: 0 }}>
          {text.intro}
        </p>
        <TextField
          id="unlock-password"
          label={t.vault.password}
          secret
          autoFocus
          value={password}
          disabled={busy}
          onChange={(e) => {
            setPassword(e.target.value);
          }}
        />
        {error && (
          <p className={s.error} role="alert">
            {error}
          </p>
        )}
        <Button type="submit" variant="primary" disabled={busy || !password}>
          {busy ? text.busy : text.action}
        </Button>
        <p className="muted" style={{ margin: 0, fontSize: 12 }}>
          {t.vault.lockedHint}
        </p>
        <button
          type="button"
          className={s.linkButton}
          onClick={() => {
            setErasing(true);
          }}
        >
          {t.vault.forgot}
        </button>
      </form>
      {erasing && (
        <EraseVaultDialog
          onClose={() => {
            setErasing(false);
          }}
        />
      )}
    </>
  );
}

/** Foca o campo de senha do painel (ex.: ao tentar reconectar com cofre bloqueado). */
export function focusUnlock(): void {
  document.getElementById('unlock-password')?.focus();
}
