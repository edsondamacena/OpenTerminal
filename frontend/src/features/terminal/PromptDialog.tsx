// Prompts enviados pelo servidor (keyboard-interactive, senha "perguntar") — FR-014.
import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useInteractions } from '../../stores/interactions';
import { useSessions } from '../../stores/sessions';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { focusTerminal } from './terminalRegistry';
import { TextField } from '../../ui/TextField';

export function PromptDialog() {
  const event = useInteractions((st) => st.prompts[0]);
  const answer = useInteractions((st) => st.answerPrompt);
  const tab = useSessions((st) => st.tabs.find((tb) => tb.id === event?.sessionId));
  const [values, setValues] = useState<string[]>([]);

  if (!event) return null;
  const reply = (answers: string[] | null) => {
    setValues([]);
    answer(event.sessionId, answers);
  };

  return (
    <Dialog
      key={`${event.sessionId}-${event.prompts.map((p) => p.text).join('|')}`}
      title={event.name || t.terminal.promptTitle}
      returnFocus={() => {
        focusTerminal(event.sessionId);
      }}
      onClose={() => {
        reply(null);
      }}
      onSubmit={() => {
        reply(event.prompts.map((_, i) => values[i] ?? ''));
      }}
      actions={
        <>
          <Button
            onClick={() => {
              reply(null);
            }}
          >
            {t.common.cancel}
          </Button>
          <Button type="submit" variant="primary">
            {t.terminal.send}
          </Button>
        </>
      }
    >
      {tab && <p style={{ margin: 0, fontWeight: 600 }}>{tab.title}</p>}
      {event.instructions && (
        <p style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{event.instructions}</p>
      )}
      {event.prompts.map((p, i) => (
        <TextField
          key={`${String(i)}-${p.text}`}
          label={p.text.trim()}
          secret={!p.echo}
          autoFocus={i === 0}
          value={values[i] ?? ''}
          onChange={(e) => {
            const next = [...values];
            next[i] = e.target.value;
            setValues(next);
          }}
        />
      ))}
    </Dialog>
  );
}
