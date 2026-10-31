// Confirmação antes de colar várias linhas (FR-144): contagem e início do conteúdo.
import { t } from '../../i18n/pt-BR';
import { useWorkspace } from '../../stores/workspace';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { describePaste } from './paste';
import { focusTerminal, terminalOf } from './terminalRegistry';
import s from './terminal.module.css';

export function PasteConfirmDialog() {
  const pending = useWorkspace((w) => w.pendingPaste);
  const setPending = useWorkspace((w) => w.setPendingPaste);
  if (!pending) return null;

  const { lines, preview } = describePaste(pending.text);
  const { sessionId, text } = pending;
  const cancel = () => {
    setPending(null);
  };
  const confirm = () => {
    setPending(null);
    terminalOf(sessionId)?.term.paste(text);
  };

  return (
    <Dialog
      title={t.terminal.pasteConfirm.title}
      width={560}
      onClose={cancel}
      onSubmit={confirm}
      returnFocus={() => {
        focusTerminal(sessionId);
      }}
      actions={
        <>
          <Button onClick={cancel}>{t.common.cancel}</Button>
          <Button type="submit" variant="primary" autoFocus>
            {t.terminal.pasteConfirm.confirm(lines)}
          </Button>
        </>
      }
    >
      <p style={{ margin: 0 }}>{t.terminal.pasteConfirm.message(lines)}</p>
      <div>
        <div className="muted">{t.terminal.pasteConfirm.preview}</div>
        <pre className={s.pastePreview} data-testid="paste-preview">
          {preview}
        </pre>
      </div>
    </Dialog>
  );
}
