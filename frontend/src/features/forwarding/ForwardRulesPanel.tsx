// Regras de encaminhamento local de um host (FR-023–FR-025).
import { useCallback, useEffect, useState } from 'react';
import type {
  ForwardRule,
  ForwardRuleInput,
  ForwardRuleStatus,
  HostView,
} from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useForwards } from '../../stores/forwards';
import { Button } from '../../ui/Button';
import { ConfirmDialog } from '../../ui/ConfirmDialog';
import { Dialog } from '../../ui/Dialog';
import { NumberField } from '../../ui/NumberField';
import { TextField } from '../../ui/TextField';
import { toastError } from '../../ui/Toast';
import { describeRule, isLoopback, statusText } from './format';
import s from './forwarding.module.css';

type RuleRow = ForwardRule & ForwardRuleStatus;

interface Draft {
  id: string | null;
  label: string;
  bindAddress: string;
  bindPort: string;
  destHost: string;
  destPort: string;
}

const EMPTY: Draft = {
  id: null,
  label: '',
  bindAddress: '127.0.0.1',
  bindPort: '',
  destHost: '',
  destPort: '',
};

const portOf = (text: string) => {
  const n = Number(text);
  return Number.isInteger(n) && n >= 1 && n <= 65535 ? n : null;
};

export function ForwardRulesPanel({ host, onClose }: { host: HostView; onClose: () => void }) {
  const statuses = useForwards((st) => st.statuses);
  const [rules, setRules] = useState<RuleRow[]>([]);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [exposing, setExposing] = useState<ForwardRuleInput | null>(null);
  const [removing, setRemoving] = useState<RuleRow | null>(null);
  const [busy, setBusy] = useState(false);
  const [reloads, setReloads] = useState(0);

  const reload = useCallback(() => {
    setReloads((n) => n + 1);
  }, []);

  useEffect(() => {
    let live = true;
    void window.openterminal.forwards.list(host.id).then((res) => {
      if (!live) return;
      if (res.ok) setRules(res.value);
      else toastError(res.error);
    });
    return () => {
      live = false;
    };
  }, [host.id, reloads]);

  // Status ao vivo vindo dos eventos sobrepõe o da última listagem.
  const rows = rules.map((r) => ({ ...r, ...(statuses[r.id] ?? {}) }));

  const bindPort = draft ? portOf(draft.bindPort) : null;
  const destPort = draft ? portOf(draft.destPort) : null;
  const bindAddress = draft?.bindAddress.trim() || '127.0.0.1';
  const duplicate =
    draft !== null &&
    bindPort !== null &&
    rows.some((r) => r.id !== draft.id && r.bindAddress === bindAddress && r.bindPort === bindPort);
  const valid =
    draft !== null &&
    bindPort !== null &&
    destPort !== null &&
    draft.destHost.trim() !== '' &&
    !duplicate;

  async function save(input: ForwardRuleInput) {
    if (!draft) return;
    setBusy(true);
    const res = draft.id
      ? await window.openterminal.forwards.update(draft.id, input)
      : await window.openterminal.forwards.create(input);
    setBusy(false);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    setDraft(null);
    reload();
  }

  function submit() {
    if (!valid) return;
    const input: ForwardRuleInput = {
      hostId: host.id,
      label: draft.label.trim() || null,
      bindAddress,
      allowNonLoopback: !isLoopback(bindAddress),
      bindPort,
      destHost: draft.destHost.trim(),
      destPort,
    };
    // Fora do loopback a porta fica exposta à rede: pede confirmação (FR-025).
    if (input.allowNonLoopback) setExposing(input);
    else void save(input);
  }

  async function toggle(rule: RuleRow) {
    const api = window.openterminal.forwards;
    const res =
      rule.status === 'active' || rule.status === 'starting'
        ? await api.deactivate(rule.id)
        : await api.activate(rule.id);
    if (!res.ok) toastError(res.error);
  }

  async function remove(rule: RuleRow) {
    setRemoving(null);
    const res = await window.openterminal.forwards.remove(rule.id);
    if (!res.ok) toastError(res.error);
    reload();
  }

  const field = (key: keyof Omit<Draft, 'id'>) => ({
    value: draft?.[key] ?? '',
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
      const value = e.target.value;
      setDraft((d) => (d ? { ...d, [key]: value } : d));
    },
  });

  const numberField = (key: 'bindPort' | 'destPort') => ({
    value: draft?.[key] ?? '',
    onChange: (value: string) => {
      setDraft((d) => (d ? { ...d, [key]: value } : d));
    },
  });

  if (exposing) {
    return (
      <ConfirmDialog
        title={t.forwarding.exposedTitle}
        message={t.forwarding.exposed(exposing.bindAddress)}
        confirmLabel={t.forwarding.exposedConfirm}
        danger
        onCancel={() => {
          setExposing(null);
        }}
        onConfirm={() => {
          const input = exposing;
          setExposing(null);
          void save(input);
        }}
      />
    );
  }

  if (removing) {
    return (
      <ConfirmDialog
        title={t.common.remove}
        message={t.forwarding.removeConfirm(describeRule(removing))}
        confirmLabel={t.common.remove}
        danger
        onCancel={() => {
          setRemoving(null);
        }}
        onConfirm={() => void remove(removing)}
      />
    );
  }

  return (
    <Dialog
      title={t.forwarding.title(host.label)}
      onClose={onClose}
      width={680}
      actions={<Button onClick={onClose}>{t.common.close}</Button>}
    >
      {rows.length === 0 ? (
        <p className="muted" style={{ margin: 0 }}>
          {t.forwarding.empty}
        </p>
      ) : (
        <ul className={s.rules} aria-label={t.forwarding.open}>
          {rows.map((rule) => {
            const on = rule.status === 'active' || rule.status === 'starting';
            return (
              <li key={rule.id} className={s.rule} aria-label={describeRule(rule)}>
                <div className={s.ruleText}>
                  {rule.label && <span className={s.ruleLabel}>{rule.label}</span>}
                  <span className={s.mono}>{describeRule(rule)}</span>
                </div>
                <span className={s.status} data-status={rule.status} role="status">
                  {statusText(rule)}
                </span>
                <Button variant={on ? 'default' : 'primary'} onClick={() => void toggle(rule)}>
                  {on ? t.forwarding.deactivate : t.forwarding.activate}
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => {
                    setDraft({
                      id: rule.id,
                      label: rule.label ?? '',
                      bindAddress: rule.bindAddress,
                      bindPort: String(rule.bindPort),
                      destHost: rule.destHost,
                      destPort: String(rule.destPort),
                    });
                  }}
                >
                  {t.common.edit}
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => {
                    setRemoving(rule);
                  }}
                >
                  {t.common.remove}
                </Button>
              </li>
            );
          })}
        </ul>
      )}

      {draft ? (
        <form
          autoComplete="off"
          className={s.form}
          aria-label={draft.id ? t.forwarding.edit : t.forwarding.add}
          onSubmit={(e) => {
            e.preventDefault();
            submit();
          }}
        >
          <h3 className={s.formTitle}>{draft.id ? t.forwarding.edit : t.forwarding.add}</h3>
          <TextField
            label={`${t.forwarding.label} (${t.common.optional})`}
            maxLength={100}
            {...field('label')}
          />
          <div className={s.row}>
            <TextField
              label={t.forwarding.bindAddress}
              hint={t.forwarding.bindAddressHint}
              {...field('bindAddress')}
            />
            <NumberField
              label={t.forwarding.bindPort}
              min={1}
              max={65535}
              error={duplicate ? t.forwarding.duplicate : null}
              {...numberField('bindPort')}
            />
          </div>
          <div className={s.row}>
            <TextField
              label={t.forwarding.destHost}
              hint={t.forwarding.destHostHint}
              {...field('destHost')}
            />
            <NumberField
              label={t.forwarding.destPort}
              min={1}
              max={65535}
              {...numberField('destPort')}
            />
          </div>
          <div className={s.formActions}>
            <Button
              onClick={() => {
                setDraft(null);
              }}
            >
              {t.common.cancel}
            </Button>
            <Button type="submit" variant="primary" disabled={busy || !valid}>
              {t.common.save}
            </Button>
          </div>
        </form>
      ) : (
        <Button
          style={{ alignSelf: 'flex-start' }}
          onClick={() => {
            setDraft(EMPTY);
          }}
        >
          {t.forwarding.add}
        </Button>
      )}
    </Dialog>
  );
}
