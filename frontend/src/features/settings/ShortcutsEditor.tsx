// Atalhos remapeáveis (FR-173): clicar na combinação grava a próxima tecla; cada ação pode ser
// desativada ou voltar ao padrão. Conflito com outra ação é recusado com o nome dela.
import { useEffect, useRef, useState } from 'react';
import {
  ACTIONS,
  comboLabel,
  comboOf,
  conflictOf,
  effectiveShortcuts,
  stealsFromTerminal,
  type Combo,
  type ShortcutAction,
  type ShortcutOverrides,
} from '../../app/shortcuts';
import { useShortcutOverrides } from '../../app/useShortcuts';
import { t } from '../../i18n/pt-BR';
import { useUi } from '../../stores/ui';
import { Button } from '../../ui/Button';
import { WarningIcon } from '../../ui/icons';
import { toastError } from '../../ui/Toast';
import s from './settings.module.css';

const ts = t.settings;

async function saveOverrides(next: ShortcutOverrides) {
  const ui = useUi.getState();
  const before = ui.settings;
  if (!before) return;
  // Vale na hora; volta atrás se o processo principal recusar.
  ui.setSettings({ ...before, shortcuts: next });
  const res = await window.openterminal.app.updateUiSettings({ shortcuts: next });
  if (res.ok) ui.setSettings(res.value);
  else {
    ui.setSettings(before);
    toastError(res.error);
  }
}

type Problem =
  | { kind: 'modifier' }
  | { kind: 'conflict'; with: ShortcutAction }
  | { kind: 'steals'; combo: Combo };

function Recorder({
  action,
  overrides,
  onDone,
}: {
  action: ShortcutAction;
  overrides: ShortcutOverrides;
  onDone: () => void;
}) {
  const [problem, setProblem] = useState<Problem | null>(null);
  const ref = useRef<HTMLDivElement>(null);

  // O React só aplica `autoFocus` em campos e botões: o gravador recebe o foco aqui.
  useEffect(() => {
    ref.current?.focus();
  }, []);

  function accept(combo: Combo) {
    void saveOverrides({ ...overrides, [action]: combo });
    onDone();
  }

  return (
    <div className={s.recorderWrap}>
      <div
        className={s.recorder}
        ref={ref}
        data-shortcut-recorder
        tabIndex={0}
        role="textbox"
        aria-label={ts.shortcutRecording}
        onBlur={onDone}
        onKeyDown={(e) => {
          e.preventDefault();
          e.stopPropagation();
          if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey) {
            onDone();
            return;
          }
          const combo = comboOf(e);
          if (!combo) return;
          const fKey = /^F\d+$/.test(e.code);
          if (!e.ctrlKey && !e.altKey && !fKey) {
            setProblem({ kind: 'modifier' });
            return;
          }
          const other = conflictOf(combo, action, overrides);
          if (other) {
            setProblem({ kind: 'conflict', with: other });
            return;
          }
          if (stealsFromTerminal(combo)) {
            setProblem({ kind: 'steals', combo });
            return;
          }
          accept(combo);
        }}
      >
        {ts.shortcutRecording}
      </div>
      {problem && (
        <p className={s.recorderProblem} role="alert">
          {problem.kind === 'modifier' && <span>{ts.shortcutNeedsModifier}</span>}
          {problem.kind === 'conflict' && (
            <span>{ts.shortcutConflict(ts.shortcutActions[problem.with])}</span>
          )}
          {problem.kind === 'steals' && (
            <>
              <WarningIcon aria-hidden />
              <span>{ts.shortcutSteals(comboLabel(problem.combo))}</span>
              <Button
                variant="ghost"
                // Sem tirar o foco do gravador antes do clique valer.
                onMouseDown={(e) => {
                  e.preventDefault();
                }}
                onClick={() => {
                  accept(problem.combo);
                }}
              >
                {ts.shortcutUseAnyway}
              </Button>
            </>
          )}
        </p>
      )}
    </div>
  );
}

export function ShortcutsEditor() {
  const overrides = useShortcutOverrides();
  const effective = effectiveShortcuts(overrides);
  const [recording, setRecording] = useState<ShortcutAction | null>(null);
  const customized = Object.keys(overrides).length > 0;

  return (
    <>
      <p className={s.hint}>{ts.shortcutsIntro}</p>
      <ul className={s.shortcutList} aria-label={t.settings.sections.shortcuts}>
        {ACTIONS.map((action) => {
          const combos = effective[action];
          const changed = overrides[action] !== undefined;
          const name = ts.shortcutActions[action];
          return (
            <li key={action} className={s.shortcutRow} data-changed={changed || undefined}>
              <span className={s.shortcutName}>{name}</span>
              {recording === action ? (
                <Recorder
                  action={action}
                  overrides={overrides}
                  onDone={() => {
                    setRecording(null);
                  }}
                />
              ) : (
                <button
                  type="button"
                  className={s.shortcutKeys}
                  aria-label={ts.shortcutChange(name)}
                  title={ts.shortcutChange(name)}
                  onClick={() => {
                    setRecording(action);
                  }}
                >
                  {combos.length === 0 ? (
                    <span className={s.shortcutOff}>{ts.shortcutDisabled}</span>
                  ) : (
                    combos.slice(0, 2).map((combo) => (
                      <kbd key={combo} className={s.kbd}>
                        {comboLabel(combo)}
                      </kbd>
                    ))
                  )}
                </button>
              )}
              <span className={s.shortcutActions}>
                {combos.length > 0 && (
                  <Button
                    variant="ghost"
                    aria-label={`${ts.shortcutDisable}: ${name}`}
                    onClick={() => void saveOverrides({ ...overrides, [action]: '' })}
                  >
                    {ts.shortcutDisable}
                  </Button>
                )}
                {changed && (
                  <Button
                    variant="ghost"
                    aria-label={`${ts.shortcutReset}: ${name}`}
                    onClick={() => {
                      void saveOverrides(
                        Object.fromEntries(Object.entries(overrides).filter(([a]) => a !== action)),
                      );
                    }}
                  >
                    {ts.shortcutReset}
                  </Button>
                )}
              </span>
            </li>
          );
        })}
      </ul>
      {customized && (
        <Button style={{ alignSelf: 'flex-start' }} onClick={() => void saveOverrides({})}>
          {ts.shortcutResetAll}
        </Button>
      )}
      <h3 className={s.subTitle}>{ts.shortcutFixedTitle}</h3>
      <ul className={s.shortcutList}>
        {ts.shortcutFixed.map(([keys, label]) => (
          <li key={keys} className={s.shortcutRow}>
            <span className={s.shortcutName}>{label}</span>
            <span className={s.shortcutKeysStatic}>
              <kbd className={s.kbd}>{keys}</kbd>
            </span>
          </li>
        ))}
      </ul>
    </>
  );
}
