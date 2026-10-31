// Editor de esquema (FR-153): nome e as 20 cores por seletor de cor, com prévia na hora.
import { useState } from 'react';
import type { ColorScheme } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { Button } from '../../ui/Button';
import { Dialog } from '../../ui/Dialog';
import { TextField } from '../../ui/TextField';
import { toastError } from '../../ui/Toast';
import s from './terminalSettings.module.css';

const tt = t.terminalSettings;

/** O seletor nativo só aceita `#rrggbb`; a seleção pode ter alfa (`#rrggbbaa`). */
function rgbOf(color: string): string {
  return color.slice(0, 7);
}

function ColorInput({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className={s.colorInput} title={label}>
      <input
        type="color"
        value={rgbOf(value)}
        aria-label={label}
        onChange={(e) => {
          onChange(e.target.value);
        }}
      />
      <span>{label}</span>
    </label>
  );
}

export function SchemeEditor({
  scheme,
  onClose,
  onSaved,
}: {
  scheme: ColorScheme;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<ColorScheme>({ ...scheme, ansi: [...scheme.ansi] });
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<ColorScheme>) => {
    setDraft((d) => ({ ...d, ...patch }));
  };
  const setAnsi = (i: number, color: string) => {
    setDraft((d) => ({ ...d, ansi: d.ansi.map((c, n) => (n === i ? color : c)) }));
  };
  // Seleção mantém o alfa original ao trocar só a cor.
  const alpha = scheme.selection.length === 9 ? scheme.selection.slice(7) : '';

  async function save() {
    setBusy(true);
    const res = await window.openterminal.profiles.saveScheme({ ...draft, name: draft.name.trim() });
    setBusy(false);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    onSaved();
    onClose();
  }

  const [black, red, green, yellow, blue] = draft.ansi;

  return (
    <Dialog
      title={tt.editorTitle}
      onClose={onClose}
      width={620}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button
            variant="primary"
            disabled={busy || draft.name.trim() === ''}
            onClick={() => void save()}
          >
            {t.common.save}
          </Button>
        </>
      }
    >
      <TextField
        label={tt.schemeName}
        maxLength={100}
        value={draft.name}
        onChange={(e) => {
          set({ name: e.target.value });
        }}
      />
      <div
        className={s.editorSample}
        style={{ background: draft.background, color: draft.foreground }}
        aria-hidden
      >
        <span>
          <span style={{ color: green }}>noc@olt</span>:<span style={{ color: blue }}>~</span>${' '}
          <span style={{ background: draft.selection, color: draft.foreground }}>show running</span>
          <span style={{ background: draft.cursor, color: draft.background }}> </span>
        </span>
        <span>
          <span style={{ color: red }}>% Invalid input</span>{' '}
          <span style={{ color: yellow }}>warning</span> <span style={{ color: black }}>■</span>
        </span>
      </div>
      <div className={s.colorGrid}>
        <ColorInput
          label={tt.colorForeground}
          value={draft.foreground}
          onChange={(foreground) => {
            set({ foreground });
          }}
        />
        <ColorInput
          label={tt.colorBackground}
          value={draft.background}
          onChange={(background) => {
            set({ background });
          }}
        />
        <ColorInput
          label={tt.colorCursor}
          value={draft.cursor}
          onChange={(cursor) => {
            set({ cursor });
          }}
        />
        <ColorInput
          label={tt.colorSelection}
          value={draft.selection}
          onChange={(selection) => {
            set({ selection: selection + alpha });
          }}
        />
      </div>
      <h4 className={s.editorHeading}>{tt.normalColors}</h4>
      <div className={s.colorGrid}>
        {draft.ansi.slice(0, 8).map((color, i) => (
          <ColorInput
            key={i}
            label={tt.ansiNames[i] ?? ''}
            value={color}
            onChange={(c) => {
              setAnsi(i, c);
            }}
          />
        ))}
      </div>
      <h4 className={s.editorHeading}>{tt.brightColors}</h4>
      <div className={s.colorGrid}>
        {draft.ansi.slice(8).map((color, i) => (
          <ColorInput
            key={i}
            label={tt.ansiNames[i + 8] ?? ''}
            value={color}
            onChange={(c) => {
              setAnsi(i + 8, c);
            }}
          />
        ))}
      </div>
    </Dialog>
  );
}
