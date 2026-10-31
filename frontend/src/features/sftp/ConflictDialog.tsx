// Conflito de nome na transferência (FR-021): substituir, pular ou cancelar o restante.
import { fmtBytes, fmtDate, t } from '../../i18n/pt-BR';
import { useTransfers } from '../../stores/transfers';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';

export function ConflictDialog() {
  const conflict = useTransfers((st) => st.conflicts[0]);
  const answer = useTransfers((st) => st.answerConflict);
  if (!conflict) return null;
  const reply = (choice: 'replace' | 'skip' | 'cancelAll') => {
    answer(conflict.transferId, choice);
  };

  return (
    <Dialog
      key={conflict.transferId}
      title={t.sftp.conflictTitle}
      onClose={() => {
        reply('cancelAll');
      }}
      actions={
        <>
          <Button
            onClick={() => {
              reply('cancelAll');
            }}
          >
            {t.sftp.cancelAll}
          </Button>
          <Button
            autoFocus
            onClick={() => {
              reply('skip');
            }}
          >
            {t.sftp.skip}
          </Button>
          <Button
            variant="primary"
            onClick={() => {
              reply('replace');
            }}
          >
            {t.sftp.replace}
          </Button>
        </>
      }
    >
      <p style={{ margin: 0 }}>{t.sftp.conflict(conflict.name)}</p>
      <p className="muted" style={{ margin: 0 }}>
        {t.sftp.existing}: {fmtBytes(conflict.existingSize)} ·{' '}
        {fmtDate(conflict.existingModifiedAt)}
      </p>
    </Dialog>
  );
}
