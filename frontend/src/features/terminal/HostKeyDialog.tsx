// Confirmação da chave do servidor (FR-013). Divergência exige aceite consciente.
import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useInteractions } from '../../stores/interactions';
import { useSessions } from '../../stores/sessions';
import { Button } from '../../ui/Button';
import { Checkbox } from '../../ui/Checkbox';
import { Dialog } from '../../ui/Dialog';
import { focusTerminal } from './terminalRegistry';
import { WarningIcon } from '../../ui/icons';
import s from './terminal.module.css';

export function HostKeyDialog() {
  const event = useInteractions((st) => st.hostKeys[0]);
  const answer = useInteractions((st) => st.answerHostKey);
  const tab = useSessions((st) => st.tabs.find((tb) => tb.id === event?.sessionId));
  const [understood, setUnderstood] = useState(false);

  if (!event) return null;
  const changed = event.kind === 'changed';
  const reply = (accept: boolean) => {
    setUnderstood(false);
    answer(event.sessionId, accept);
  };

  return (
    <Dialog
      key={event.sessionId}
      title={changed ? t.terminal.hostKeyChangedTitle : t.terminal.hostKeyNewTitle}
      modal
      returnFocus={() => {
        focusTerminal(event.sessionId);
      }}
      onClose={() => {
        reply(false);
      }}
      width={540}
      actions={
        <>
          <Button
            autoFocus
            onClick={() => {
              reply(false);
            }}
          >
            {t.common.cancel}
          </Button>
          <Button
            variant={changed ? 'danger' : 'primary'}
            disabled={changed && !understood}
            onClick={() => {
              reply(true);
            }}
          >
            {changed ? t.terminal.acceptChangedKey : t.terminal.acceptKey}
          </Button>
        </>
      }
    >
      {tab && <p style={{ margin: 0, fontWeight: 600 }}>{tab.title}</p>}
      {changed ? (
        <div className={s.alert} role="alert">
          <WarningIcon style={{ flex: 'none' }} />
          <p>{t.terminal.hostKeyChanged}</p>
        </div>
      ) : (
        <p style={{ margin: 0 }}>{t.terminal.hostKeyNew}</p>
      )}
      <div>
        <div className="muted">
          {t.terminal.keyType}: {event.keyType}
        </div>
      </div>
      {changed && event.previousFingerprint && (
        <div>
          <div className="muted">{t.terminal.previousFingerprint}</div>
          <div className={s.fingerprint} data-testid="previous-fingerprint">
            {event.previousFingerprint}
          </div>
        </div>
      )}
      <div>
        <div className="muted">{t.terminal.newFingerprint}</div>
        <div className={s.fingerprint} data-testid="fingerprint">
          {event.fingerprint}
        </div>
      </div>
      {changed && (
        <Checkbox
          label={t.terminal.understandRisk}
          checked={understood}
          onChange={(e) => {
            setUnderstood(e.target.checked);
          }}
        />
      )}
    </Dialog>
  );
}
