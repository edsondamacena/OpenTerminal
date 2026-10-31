import { t } from '../i18n/pt-BR';
import { Button } from './Button';
import { Dialog } from './Dialog';

export interface ConfirmDialogProps {
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({
  title,
  message,
  confirmLabel = t.common.confirm,
  danger = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <Dialog
      title={title}
      onClose={onCancel}
      onSubmit={onConfirm}
      actions={
        <>
          <Button onClick={onCancel}>{t.common.cancel}</Button>
          <Button type="submit" variant={danger ? 'danger' : 'primary'} autoFocus>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <p style={{ margin: 0 }}>{message}</p>
    </Dialog>
  );
}
