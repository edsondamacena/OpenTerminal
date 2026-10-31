// Confirmação ao fechar com atividade em andamento (FR-028).
import { useEffect, useState } from 'react';
import type { ActiveSummary } from '@shared/model/types';
import { t } from '../i18n/pt-BR';
import { Button } from '../ui/Button';
import { Dialog } from '../ui/Dialog';

export function CloseConfirmDialog() {
  const [summary, setSummary] = useState<ActiveSummary | null>(null);

  useEffect(() => window.openterminal.app.onCloseRequested(setSummary), []);

  if (!summary) return null;

  const answer = (confirmed: boolean) => {
    setSummary(null);
    void window.openterminal.app.confirmClose(confirmed);
  };
  const items = [
    summary.sessions > 0 && t.closing.sessions(summary.sessions),
    summary.transfers > 0 && t.closing.transfers(summary.transfers),
    summary.forwards > 0 && t.closing.forwards(summary.forwards),
  ].filter((item) => item !== false);

  return (
    <Dialog
      title={t.closing.title}
      onClose={() => {
        answer(false);
      }}
      onSubmit={() => {
        answer(true);
      }}
      actions={
        <>
          <Button
            onClick={() => {
              answer(false);
            }}
          >
            {t.common.cancel}
          </Button>
          <Button type="submit" variant="danger" autoFocus>
            {t.closing.confirm}
          </Button>
        </>
      }
    >
      <p style={{ margin: 0 }}>{t.closing.intro}</p>
      <ul style={{ margin: 0, paddingLeft: 20 }}>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </Dialog>
  );
}
