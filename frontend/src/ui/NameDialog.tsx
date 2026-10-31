// Diálogo de um campo de nome (criar/renomear).
import { useState } from 'react';
import { t } from '../i18n/pt-BR';
import { Button } from './Button';
import { Dialog } from './Dialog';
import { TextField } from './TextField';

export interface NameDialogProps {
  title: string;
  label: string;
  initial?: string;
  maxLength?: number;
  /** Ao focar, seleciona o nome sem a extensão (arquivos). */
  selectStem?: boolean;
  /** Mensagem de erro para o nome aparado, ou `null` se válido. */
  validate?: (name: string) => string | null;
  /** Resolve `true` para fechar o diálogo. */
  onSubmit: (name: string) => Promise<boolean>;
  onClose: () => void;
}

export function NameDialog({
  title,
  label,
  initial = '',
  maxLength = 255,
  selectStem = false,
  validate,
  onSubmit,
  onClose,
}: NameDialogProps) {
  const [name, setName] = useState(initial);
  const [busy, setBusy] = useState(false);
  const clean = name.trim();
  const error = clean === '' ? null : (validate?.(clean) ?? null);
  const valid = clean !== '' && clean.length <= maxLength && error === null;

  async function submit() {
    if (!valid) return;
    setBusy(true);
    const done = await onSubmit(clean);
    setBusy(false);
    if (done) onClose();
  }

  return (
    <Dialog
      title={title}
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
        label={label}
        autoFocus
        maxLength={maxLength}
        value={name}
        error={error}
        onFocus={(e) => {
          if (!selectStem) return;
          const dot = e.currentTarget.value.lastIndexOf('.');
          e.currentTarget.setSelectionRange(0, dot > 0 ? dot : e.currentTarget.value.length);
        }}
        onChange={(e) => {
          setName(e.target.value);
        }}
      />
    </Dialog>
  );
}
