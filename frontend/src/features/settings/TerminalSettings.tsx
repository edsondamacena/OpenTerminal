// Personalização do terminal (FR-151–FR-155, FR-158a): tudo por seleção, com amostra ao vivo.
// Esquemas numa galeria com prévia, fontes desenhadas na própria fonte, cursor ilustrado e
// controles deslizantes. As mudanças valem na hora para a amostra e para as abas abertas.
import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import type { ColorScheme, FontInfo, TerminalProfile } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useProfiles } from '../../stores/profiles';
import { Button } from '../../ui/Button';
import { Checkbox } from '../../ui/Checkbox';
import { ConfirmDialog } from '../../ui/ConfirmDialog';
import { CheckIcon, WarningIcon } from '../../ui/icons';
import { Segmented } from '../../ui/Segmented';
import { Slider } from '../../ui/Slider';
import { toastError, toastInfo } from '../../ui/Toast';
import { applyProfile, cssFontFamily, schemeOf } from '../terminal/profile';
import { SchemeEditor } from './SchemeEditor';
import s from './terminalSettings.module.css';

const tt = t.terminalSettings;

/** Usadas até a lista de fontes instaladas chegar do processo principal. */
const FALLBACK_FONTS: FontInfo[] = [
  { family: 'JetBrains Mono', source: 'bundled', ligatures: true },
  { family: 'Cascadia Code', source: 'bundled', ligatures: true },
  { family: 'Cascadia Mono', source: 'system', ligatures: false },
  { family: 'Consolas', source: 'system', ligatures: false },
];

const E = '\x1b[';
const SAMPLE = [
  `${E}1;32mnoc@core-router-01${E}0m:${E}1;34m~/configs${E}0m$ ls -la`,
  `drwxr-xr-x  ${E}1;34mbackups${E}0m  ${E}1;32mdeploy.sh${E}0m  ${E}31merror.log${E}0m  ${E}33mrouter.cfg${E}0m`,
  `${E}1;32mnoc@core-router-01${E}0m:${E}1;34m~/configs${E}0m$ ping -c 1 10.0.0.1`,
  `64 bytes from 10.0.0.1: icmp_seq=1 ttl=64 time=${E}1;32m0.42 ms${E}0m`,
  `${E}2m# Paleta ANSI${E}0m`,
  Array.from({ length: 8 }, (_, i) => `${E}4${String(i)}m   `).join('') + `${E}0m`,
  Array.from({ length: 8 }, (_, i) => `${E}10${String(i)}m   `).join('') + `${E}0m`,
  `${E}1;32mnoc@core-router-01${E}0m:${E}1;34m~${E}0m$ `,
].join('\r\n');

const SCROLLBACK = [1_000, 5_000, 10_000, 50_000, 100_000] as const;

function formatLines(n: number): string {
  return n.toLocaleString('pt-BR');
}

/** Terminal de amostra: xterm de verdade, sem sessão, com o perfil aplicado a cada mudança. */
function Preview({ profile, scheme }: { profile: TerminalProfile; scheme: ColorScheme }) {
  const ref = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);

  useEffect(() => {
    const container = ref.current;
    if (!container) return;
    const term = new Terminal({
      rows: 9,
      cols: 72,
      disableStdin: true,
      allowProposedApi: true,
    });
    term.open(container);
    term.write(SAMPLE);
    termRef.current = term;
    return () => {
      term.dispose();
      termRef.current = null;
    };
  }, []);

  useEffect(() => {
    const term = termRef.current;
    if (term) void applyProfile(term, profile, scheme, profile.fontSize);
  }, [profile, scheme]);

  return (
    <div
      className={s.preview}
      aria-label={tt.preview}
      role="img"
      style={{
        padding: profile.padding,
        background: scheme.background,
        opacity: 1,
      }}
    >
      <div ref={ref} className={s.previewTerm} />
    </div>
  );
}

/** Miniatura de um esquema: prompt colorido e as oito cores normais. */
function SchemeCard({
  scheme,
  selected,
  onSelect,
}: {
  scheme: ColorScheme;
  selected: boolean;
  onSelect: () => void;
}) {
  const [black, red, green, yellow, blue, magenta, cyan] = scheme.ansi;
  const origin =
    scheme.origin === 'builtin'
      ? tt.builtin
      : scheme.origin === 'imported'
        ? tt.imported
        : tt.custom;
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      aria-label={scheme.name}
      className={s.schemeCard}
      onClick={onSelect}
    >
      <span
        className={s.schemeSample}
        style={{ background: scheme.background, color: scheme.foreground }}
      >
        <span>
          <span style={{ color: green }}>noc@olt</span>:<span style={{ color: blue }}>~</span>$ ls
        </span>
        <span>
          <span style={{ color: blue }}>logs</span> <span style={{ color: green }}>run.sh</span>{' '}
          <span style={{ color: red }}>err</span> <span style={{ color: yellow }}>cfg</span>
        </span>
        <span className={s.schemeDots}>
          {[black, red, green, yellow, blue, magenta, cyan, scheme.foreground].map((color, i) => (
            <span key={i} style={{ background: color }} />
          ))}
        </span>
      </span>
      <span className={s.schemeMeta}>
        <span className={s.schemeName}>{scheme.name}</span>
        <span className={s.schemeOrigin}>{origin}</span>
      </span>
      {selected && (
        <span className={s.schemeCheck} aria-hidden>
          <CheckIcon />
        </span>
      )}
    </button>
  );
}

function CursorShape({ style }: { style: TerminalProfile['cursorStyle'] }) {
  return (
    <span className={s.cursorShape} data-style={style} aria-hidden>
      <span>A</span>
    </span>
  );
}

export interface TerminalSettingsProps {
  profile: TerminalProfile;
  onChange: (patch: Partial<TerminalProfile>) => void;
}

/** Editor de um perfil (o global nas configurações, ou o próprio de um host). */
export function TerminalSettings({ profile, onChange: update }: TerminalSettingsProps) {
  const schemes = useProfiles((p) => p.schemes);
  const loadedFonts = useProfiles((p) => p.fonts);
  const loaded = useProfiles((p) => p.loaded);
  const reloadSchemes = useProfiles((p) => p.reloadSchemes);
  const scheme = schemeOf(schemes, profile.colorSchemeId);
  const fonts = loadedFonts.length > 0 ? loadedFonts : FALLBACK_FONTS;
  const font = fonts.find((f) => f.family === profile.fontFamily);
  const [editing, setEditing] = useState<ColorScheme | null>(null);
  const [removing, setRemoving] = useState<ColorScheme | null>(null);

  async function importScheme() {
    const res = await window.openterminal.profiles.importScheme();
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    if (!res.value) return;
    await reloadSchemes();
    const first = res.value.imported[0];
    if (first) update({ colorSchemeId: first.id });
    toastInfo(
      [
        tt.importedCount(res.value.imported.length),
        res.value.completed.length > 0 ? tt.completed(res.value.completed.join(', ')) : '',
      ]
        .filter(Boolean)
        .join(' '),
    );
  }

  async function duplicate() {
    const res = await window.openterminal.profiles.duplicateScheme(
      scheme.id,
      tt.duplicateName(scheme.name),
    );
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    await reloadSchemes();
    update({ colorSchemeId: res.value.id });
    setEditing(res.value);
  }

  async function exportScheme() {
    const res = await window.openterminal.profiles.exportScheme(scheme.id);
    if (!res.ok) toastError(res.error);
  }

  async function remove(target: ColorScheme) {
    const res = await window.openterminal.profiles.removeScheme(target.id);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    await reloadSchemes();
  }

  const bundled = fonts.filter((f) => f.source === 'bundled');
  const system = fonts.filter((f) => f.source === 'system');

  return (
    <div className={s.layout}>
      <div className={s.previewWrap}>
        <Preview profile={profile} scheme={scheme} />
      </div>

      <section className={s.group} aria-labelledby="ts-schemes">
        <div className={s.groupHead}>
          <h3 id="ts-schemes" className={s.groupTitle}>
            {tt.schemes}
          </h3>
          <div className={s.groupActions}>
            <Button variant="ghost" title={tt.importHint} onClick={() => void importScheme()}>
              {tt.import}
            </Button>
            <Button variant="ghost" onClick={() => void exportScheme()}>
              {tt.export}
            </Button>
          </div>
        </div>
        <div className={s.schemeGrid} role="radiogroup" aria-labelledby="ts-schemes">
          {schemes.map((item) => (
            <SchemeCard
              key={item.id}
              scheme={item}
              selected={item.id === scheme.id}
              onSelect={() => {
                update({ colorSchemeId: item.id });
              }}
            />
          ))}
        </div>
        <div className={s.groupActions}>
          {scheme.origin === 'builtin' ? (
            <Button onClick={() => void duplicate()}>{tt.duplicate}</Button>
          ) : (
            <>
              <Button
                onClick={() => {
                  setEditing(scheme);
                }}
              >
                {tt.edit}
              </Button>
              <Button onClick={() => void duplicate()}>{tt.duplicate}</Button>
              <Button
                variant="danger"
                onClick={() => {
                  setRemoving(scheme);
                }}
              >
                {tt.remove}
              </Button>
            </>
          )}
        </div>
      </section>

      <section className={s.group} aria-labelledby="ts-font">
        <h3 id="ts-font" className={s.groupTitle}>
          {tt.font}
        </h3>
        <div className={s.fontList} role="radiogroup" aria-labelledby="ts-font">
          {[...bundled, ...system].map((f) => {
            const selected = f.family === profile.fontFamily;
            return (
              <button
                key={`${f.source}-${f.family}`}
                type="button"
                role="radio"
                aria-checked={selected}
                aria-label={f.family}
                className={s.fontItem}
                onClick={() => {
                  update({
                    fontFamily: f.family,
                    ...(f.ligatures ? {} : { ligatures: false }),
                  });
                }}
              >
                <span className={s.fontName}>
                  {f.family}
                  {f.source === 'bundled' && <span className={s.badge}>{tt.bundled}</span>}
                </span>
                <span className={s.fontSample} style={{ fontFamily: cssFontFamily(f.family) }}>
                  {tt.fontSample}
                </span>
                {selected && <CheckIcon className={s.fontCheck} aria-hidden />}
              </button>
            );
          })}
          {!loaded && <p className={s.muted}>{tt.fontsLoading}</p>}
        </div>
        <div className={s.controls}>
          <Slider
            label={tt.fontSize}
            min={8}
            max={32}
            value={profile.fontSize}
            format={(v) => `${String(v)} px`}
            onChange={(fontSize) => {
              update({ fontSize });
            }}
          />
          <Slider
            label={tt.lineHeight}
            min={1}
            max={2}
            step={0.05}
            value={profile.lineHeight}
            format={(v) => v.toFixed(2).replace('.', ',')}
            onChange={(lineHeight) => {
              update({ lineHeight });
            }}
          />
          <Slider
            label={tt.letterSpacing}
            min={-2}
            max={10}
            step={0.5}
            value={profile.letterSpacing}
            format={(v) => `${String(v).replace('.', ',')} px`}
            onChange={(letterSpacing) => {
              update({ letterSpacing });
            }}
          />
        </div>
        <Segmented
          label={tt.weight}
          value={profile.fontWeight}
          options={[300, 400, 500].map((w) => ({
            value: w,
            label: <span style={{ fontWeight: w }}>{tt.weights[w as 300 | 400 | 500]}</span>,
          }))}
          onChange={(fontWeight) => {
            update({ fontWeight });
          }}
        />
        <Segmented
          label={tt.weightBold}
          value={profile.fontWeightBold}
          options={[600, 700, 800].map((w) => ({
            value: w,
            label: <span style={{ fontWeight: w }}>{tt.weights[w as 600 | 700 | 800]}</span>,
          }))}
          onChange={(fontWeightBold) => {
            update({ fontWeightBold });
          }}
        />
        <Checkbox
          label={tt.ligatures}
          hint={font && !font.ligatures ? tt.ligaturesUnsupported : undefined}
          disabled={font ? !font.ligatures : false}
          checked={profile.ligatures}
          onChange={(e) => {
            update({ ligatures: e.target.checked });
          }}
        />
      </section>

      <section className={s.group} aria-labelledby="ts-cursor">
        <h3 id="ts-cursor" className={s.groupTitle}>
          {tt.cursor}
        </h3>
        <Segmented
          label={tt.cursorStyle}
          value={profile.cursorStyle}
          options={(
            [
              ['block', tt.cursorBlock],
              ['bar', tt.cursorBar],
              ['underline', tt.cursorUnderline],
            ] as const
          ).map(([value, label]) => ({
            value,
            label: (
              <>
                <CursorShape style={value} />
                {label}
              </>
            ),
          }))}
          onChange={(cursorStyle) => {
            update({ cursorStyle });
          }}
        />
        <Checkbox
          label={tt.cursorBlink}
          checked={profile.cursorBlink}
          onChange={(e) => {
            update({ cursorBlink: e.target.checked });
          }}
        />
        <div className={s.swatchField}>
          <span className={s.label} id="ts-cursor-color">
            {tt.cursorColor}
          </span>
          <div className={s.swatches} role="radiogroup" aria-labelledby="ts-cursor-color">
            <button
              type="button"
              role="radio"
              aria-checked={profile.cursorColor === null}
              className={s.swatchAuto}
              onClick={() => {
                update({ cursorColor: null });
              }}
            >
              <span className={s.swatchDot} style={{ background: scheme.cursor }} />
              {tt.cursorFromScheme}
            </button>
            {scheme.ansi.slice(9, 15).map((color, i) => (
              <button
                key={color + String(i)}
                type="button"
                role="radio"
                aria-checked={profile.cursorColor === color}
                aria-label={tt.ansiNames[i + 9]}
                title={tt.ansiNames[i + 9]}
                className={s.swatch}
                style={{ background: color }}
                onClick={() => {
                  update({ cursorColor: color });
                }}
              />
            ))}
          </div>
        </div>
      </section>

      <section className={s.group} aria-labelledby="ts-background">
        <h3 id="ts-background" className={s.groupTitle}>
          {tt.background}
        </h3>
        <div className={s.controls}>
          <Slider
            label={tt.opacity}
            min={50}
            max={100}
            step={5}
            value={Math.round(profile.backgroundOpacity * 100)}
            format={(v) => `${String(v)}%`}
            onChange={(v) => {
              update({ backgroundOpacity: v / 100 });
            }}
          />
          <Slider
            label={tt.blur}
            min={0}
            max={40}
            value={profile.blur}
            disabled={profile.backgroundOpacity >= 1}
            hint={profile.backgroundOpacity >= 1 ? tt.blurDisabled : undefined}
            format={(v) => `${String(v)} px`}
            onChange={(blur) => {
              update({ blur });
            }}
          />
          <Slider
            label={tt.padding}
            min={0}
            max={32}
            value={profile.padding}
            format={(v) => `${String(v)} px`}
            onChange={(padding) => {
              update({ padding });
            }}
          />
        </div>
        {profile.backgroundOpacity < 1 && (
          <p className={s.warning} role="note">
            <WarningIcon />
            {tt.opacityWarning}
          </p>
        )}
        <Segmented
          label={tt.scrollback}
          value={
            SCROLLBACK.includes(profile.scrollback as (typeof SCROLLBACK)[number])
              ? profile.scrollback
              : 10_000
          }
          options={SCROLLBACK.map((n) => ({ value: n, label: formatLines(n) }))}
          onChange={(scrollback) => {
            update({ scrollback });
          }}
        />
      </section>

      {editing && (
        <SchemeEditor
          scheme={editing}
          onClose={() => {
            setEditing(null);
          }}
          onSaved={() => void reloadSchemes()}
        />
      )}
      {removing && (
        <ConfirmDialog
          title={tt.remove}
          message={tt.removeConfirm(removing.name)}
          confirmLabel={tt.remove}
          danger
          onCancel={() => {
            setRemoving(null);
          }}
          onConfirm={() => {
            const target = removing;
            setRemoving(null);
            void remove(target);
          }}
        />
      )}
    </div>
  );
}

/** Perfil global, salvo automaticamente. */
export function GlobalTerminalSettings() {
  const profile = useProfiles((p) => p.global);
  const update = useProfiles((p) => p.updateGlobal);
  return <TerminalSettings profile={profile} onChange={update} />;
}
