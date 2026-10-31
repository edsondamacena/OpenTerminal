// Encaminhamentos abertos, visíveis e desativáveis mesmo com o cofre bloqueado (FR-003).
// Aberto pelo item da barra de status (FR-172).
import { t } from '../../i18n/pt-BR';
import { useForwards } from '../../stores/forwards';
import { useWorkspace } from '../../stores/workspace';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { toastError } from '../../ui/Toast';
import { describeRule, statusText } from './format';
import s from './forwarding.module.css';

export function ActiveForwardsDialog() {
  const open = useWorkspace((w) => w.forwardsOpen);
  const setOpen = useWorkspace((w) => w.setForwardsOpen);
  const active = useForwards((st) => st.active);
  if (!open) return null;

  async function deactivate(ruleId: string) {
    const res = await window.openterminal.forwards.deactivate(ruleId);
    if (!res.ok) toastError(res.error);
  }

  return (
    <Dialog
      title={t.forwarding.activeTitle}
      onClose={() => {
        setOpen(false);
      }}
      width={600}
      actions={
        <Button
          onClick={() => {
            setOpen(false);
          }}
        >
          {t.common.close}
        </Button>
      }
    >
      {active.length === 0 ? (
        <p className="muted" style={{ margin: 0 }}>
          {t.forwarding.activeCount(0)}
        </p>
      ) : (
        <ul className={s.rules}>
          {active.map((a) => (
            <li key={a.ruleId} className={s.rule} aria-label={describeRule(a)}>
              <div className={s.ruleText}>
                <span className={s.ruleLabel}>{a.hostLabel}</span>
                <span className={s.mono}>{describeRule(a)}</span>
              </div>
              <span className={s.status} data-status={a.status} role="status">
                {statusText(a)}
              </span>
              <Button onClick={() => void deactivate(a.ruleId)}>{t.forwarding.deactivate}</Button>
            </li>
          ))}
        </ul>
      )}
    </Dialog>
  );
}
