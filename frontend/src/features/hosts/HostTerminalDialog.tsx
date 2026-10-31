// Perfil de terminal próprio de um host (FR-154): o mesmo editor das configurações, num rascunho
// que só vale para as sessões desse host depois de salvo.
import { useState } from 'react';
import type { TerminalProfile } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useProfiles } from '../../stores/profiles';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { TerminalSettings } from '../settings/TerminalSettings';
import s from './hosts.module.css';

export function HostTerminalDialog({
  hostId,
  hostLabel,
  onClose,
}: {
  hostId: string;
  hostLabel: string;
  onClose: () => void;
}) {
  const initial = useProfiles((p) => p.overrides[hostId] ?? p.global);
  const setOverride = useProfiles((p) => p.setOverride);
  const [draft, setDraft] = useState<TerminalProfile>(initial);
  const [busy, setBusy] = useState(false);

  async function save() {
    setBusy(true);
    const ok = await setOverride(hostId, { ...draft, isGlobal: false });
    setBusy(false);
    if (ok) onClose();
  }

  return (
    <Dialog
      title={t.hosts.terminalProfileTitle(hostLabel)}
      onClose={onClose}
      width={880}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button variant="primary" disabled={busy} onClick={() => void save()}>
            {t.common.save}
          </Button>
        </>
      }
    >
      <p className={s.hint}>{t.hosts.terminalProfileHint}</p>
      <div className={s.terminalEditor}>
        <TerminalSettings
          profile={draft}
          onChange={(patch) => {
            setDraft((d) => ({ ...d, ...patch }));
          }}
        />
      </div>
    </Dialog>
  );
}
