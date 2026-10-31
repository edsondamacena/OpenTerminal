import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { WarningIcon } from '../../ui/icons';
import { EraseVaultDialog } from './EraseVaultDialog';
import s from './vault.module.css';

export function UnreadableScreen() {
  const reason = useVault((v) => v.status?.reason);
  const [erasing, setErasing] = useState(false);
  return (
    <div className={s.fullscreen}>
      <div className={`${s.card} raised glass-edge`} role="alert">
        <h1 className={s.title}>
          <WarningIcon style={{ color: 'var(--danger)', marginRight: 8 }} />
          {t.vault.unreadableTitle}
        </h1>
        <p className={s.intro}>
          {reason === 'unsupportedVersion'
            ? t.vault.unreadableVersion
            : t.vault.unreadableCorrupted}
        </p>
        <div className={s.actions}>
          <Button
            variant="danger"
            onClick={() => {
              setErasing(true);
            }}
          >
            {t.vault.eraseAction}
          </Button>
        </div>
      </div>
      {erasing && (
        <EraseVaultDialog
          onClose={() => {
            setErasing(false);
          }}
        />
      )}
    </div>
  );
}
