// Diálogos de grupo: nome (criar/renomear) e destino (mover host ou grupo).
import { useState } from 'react';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { NameDialog } from '../../ui/NameDialog';
import { Select } from '../../ui/Select';
import { groupPaths, selfAndDescendants } from './tree';

export interface GroupNameDialogProps {
  title: string;
  initial?: string;
  onSubmit: (name: string) => Promise<boolean>;
  onClose: () => void;
}

export function GroupNameDialog(props: GroupNameDialogProps) {
  return <NameDialog {...props} label={t.groups.name} maxLength={100} />;
}

const ROOT = '';

export interface MoveDialogProps {
  current: string | null;
  /** Ao mover um grupo, ele e seus descendentes não são destinos válidos. */
  movingGroupId?: string;
  onSubmit: (groupId: string | null) => Promise<boolean>;
  onClose: () => void;
}

export function MoveDialog({ current, movingGroupId, onSubmit, onClose }: MoveDialogProps) {
  const groups = useHosts((h) => h.groups);
  const [target, setTarget] = useState(current ?? ROOT);
  const [busy, setBusy] = useState(false);
  const blocked = movingGroupId ? selfAndDescendants(groups, movingGroupId) : new Set<string>();
  const paths = groupPaths(groups);
  const options = [
    { value: ROOT, label: t.groups.root },
    ...groups
      .filter((g) => !blocked.has(g.id))
      .map((g) => ({ value: g.id, label: paths.get(g.id) ?? g.name }))
      .sort((a, b) => a.label.localeCompare(b.label, 'pt-BR')),
  ];

  async function submit() {
    setBusy(true);
    const done = await onSubmit(target === ROOT ? null : target);
    setBusy(false);
    if (done) onClose();
  }

  return (
    <Dialog
      title={t.groups.moveTitle}
      onClose={onClose}
      onSubmit={() => void submit()}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button type="submit" variant="primary" disabled={busy}>
            {t.common.confirm}
          </Button>
        </>
      }
    >
      <Select label={t.groups.destination} value={target} options={options} onChange={setTarget} />
    </Dialog>
  );
}
