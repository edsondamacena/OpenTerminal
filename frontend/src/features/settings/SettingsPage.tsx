// Configurações como página na área principal (FR-168): navegação por seções à esquerda,
// conteúdo à direita e salvamento automático. As abas seguem vivas por baixo.
import { useEffect, useRef, useState, type ReactNode } from 'react';
import type {
  BackspaceMode,
  BellMode,
  LocalShell,
  Settings,
  TermType,
  ThemePreference,
  VaultSettings,
} from '@shared/model/types';
import { useMobile } from '../../app/mobile';
import { t } from '../../i18n/pt-BR';
import { useSessions } from '../../stores/sessions';
import { shellInfo, useShells } from '../../stores/shells';
import { useUi } from '../../stores/ui';
import { useVault } from '../../stores/vault';
import { useWorkspace } from '../../stores/workspace';
import { DEFAULT_ACCENT, resolveAccent } from '../../theme/accent';
import { BRAND_ACCENTS } from '../../theme/brand.gen';
import { Button } from '../../ui/Button';
import { Checkbox } from '../../ui/Checkbox';
import {
  BackIcon,
  CloudIcon,
  DownloadIcon,
  GeneralIcon,
  InfoIcon,
  KeyboardIcon,
  NetworkIcon,
  PaletteIcon,
  ShieldIcon,
  TerminalIcon,
} from '../../ui/icons';
import { useSlidingIndicator } from '../../ui/motion';
import { NumberField } from '../../ui/NumberField';
import { Select } from '../../ui/Select';
import { ShellIcon } from '../../ui/ShellIcon';
import { toastError, toastInfo } from '../../ui/Toast';
import { focusTerminal } from '../terminal/terminalRegistry';
import { ChangePasswordDialog } from '../vault/ChangePasswordDialog';
import { EraseVaultDialog } from '../vault/EraseVaultDialog';
import { ShortcutsEditor } from './ShortcutsEditor';
import { GlobalTerminalSettings } from './TerminalSettings';
import { SyncSection } from './SyncSection';
import { TransferSection } from './TransferSection';
import s from './settings.module.css';

type Section = keyof typeof t.settings.sections;

const SECTIONS: { id: Section; icon: ReactNode }[] = [
  { id: 'general', icon: <GeneralIcon /> },
  { id: 'appearance', icon: <PaletteIcon /> },
  { id: 'terminal', icon: <TerminalIcon /> },
  { id: 'connection', icon: <NetworkIcon /> },
  { id: 'vault', icon: <ShieldIcon /> },
  { id: 'sync', icon: <CloudIcon /> },
  { id: 'transfer', icon: <DownloadIcon /> },
  { id: 'shortcuts', icon: <KeyboardIcon /> },
  { id: 'about', icon: <InfoIcon /> },
];

const THEMES: { value: ThemePreference; label: string }[] = [
  { value: 'system', label: t.settings.themeSystem },
  { value: 'light', label: t.settings.themeLight },
  { value: 'dark', label: t.settings.themeDark },
];

const TERMS: { value: TermType; label: string }[] = (
  ['xterm-256color', 'xterm', 'vt100', 'linux'] as const
).map((value) => ({ value, label: t.settings.termOptions[value] }));

const SAVE_DELAY_MS = 400;

function intIn(text: string, min: number, max: number): number | null {
  const n = Number(text);
  return Number.isInteger(n) && n >= min && n <= max ? n : null;
}

/** Estado local de um campo que acompanha o valor salvo quando ele muda por fora. */
function useField<T>(source: T): [T, (value: T) => void] {
  const [value, setValue] = useState(source);
  const [from, setFrom] = useState(source);
  if (source !== from) {
    setFrom(source);
    setValue(source);
  }
  return [value, setValue];
}

/** Salva `desired` depois de uma pausa na digitação, se for válido e diferente do salvo. */
function useAutosave<T>(desired: T | null, saved: T, save: (value: T) => void) {
  const key = desired === null ? null : JSON.stringify(desired);
  const savedKey = JSON.stringify(saved);
  const latest = useRef(save);
  useEffect(() => {
    latest.current = save;
  });
  useEffect(() => {
    if (key === null || key === savedKey) return;
    const timer = setTimeout(() => {
      latest.current(JSON.parse(key) as T);
    }, SAVE_DELAY_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [key, savedKey]);
}

/** Vale na tela na hora; volta atrás se o Go recusar. */
async function saveUi(patch: Partial<Omit<Settings, 'window'>>): Promise<void> {
  const ui = useUi.getState();
  const before = ui.settings;
  if (before) ui.setSettings({ ...before, ...patch });
  const res = await window.openterminal.app.updateUiSettings(patch);
  if (res.ok) useUi.getState().setSettings(res.value);
  else {
    if (before) useUi.getState().setSettings(before);
    toastError(res.error);
  }
}

function Group({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <div className={s.group}>
      {title && <h3 className={s.groupTitle}>{title}</h3>}
      {children}
    </div>
  );
}

function GeneralSection({ settings }: { settings: Settings }) {
  const shells = useShells((s) => s.shells);
  const options: { value: LocalShell; label: string; icon: ReactNode }[] = shells.map((shell) => ({
    value: shell.id,
    label: shell.label,
    icon: <ShellIcon kind={shell.kind} label={shell.label} />,
  }));
  // Shell salvo que sumiu da máquina (WSL removido, Git desinstalado) continua visível.
  if (!shells.some((shell) => shell.id === settings.defaultShell)) {
    const missing = shellInfo(settings.defaultShell);
    options.push({
      value: missing.id,
      label: t.settings.shellMissing(missing.label),
      icon: <ShellIcon kind={missing.kind} label={missing.label} />,
    });
  }
  return (
    <Group>
      <Select
        label={t.settings.defaultShell}
        value={settings.defaultShell}
        options={options}
        onChange={(defaultShell) => void saveUi({ defaultShell })}
      />
    </Group>
  );
}

const BACKSPACE: { value: BackspaceMode; label: string }[] = (
  ['auto', 'del', 'ctrlh'] as const
).map((value) => ({ value, label: t.settings.backspaceOptions[value] }));

const BELLS: { value: BellMode; label: string }[] = (['visual', 'sound', 'off'] as const).map(
  (value) => ({ value, label: t.settings.bellOptions[value] }),
);

/** Opções de FR-177; valem na hora para todas as abas. */
function TerminalBehavior({ settings }: { settings: Settings }) {
  return (
    <Group title={t.settings.behavior}>
      <Checkbox
        label={t.settings.copyOnSelect}
        hint={t.settings.copyOnSelectHint}
        checked={settings.copyOnSelect}
        onChange={(e) => void saveUi({ copyOnSelect: e.target.checked })}
      />
      <Checkbox
        label={t.settings.rightClickPaste}
        checked={settings.rightClickPaste}
        onChange={(e) => void saveUi({ rightClickPaste: e.target.checked })}
      />
      <Checkbox
        label={t.settings.boldBright}
        hint={t.settings.boldBrightHint}
        checked={settings.boldBright}
        onChange={(e) => void saveUi({ boldBright: e.target.checked })}
      />
      <Checkbox
        label={t.settings.highlight}
        hint={t.settings.highlightHint}
        checked={settings.highlight}
        onChange={(e) => void saveUi({ highlight: e.target.checked })}
      />
      <Checkbox
        label={t.settings.autoReconnect}
        hint={t.settings.autoReconnectHint}
        checked={settings.autoReconnect}
        onChange={(e) => void saveUi({ autoReconnect: e.target.checked })}
      />
      <Select
        label={t.settings.bell}
        value={settings.bell}
        options={BELLS}
        onChange={(bell) => void saveUi({ bell })}
      />
      <Select
        label={t.settings.backspace}
        value={settings.backspace}
        options={BACKSPACE}
        onChange={(backspace) => void saveUi({ backspace })}
      />
    </Group>
  );
}

/** Amostra da cor como ela fica no tema atual (a do Windows já ajustada para 4,5:1). */
function AccentSwatch({ color }: { color: string }) {
  return <span className={s.swatch} style={{ background: color }} aria-hidden="true" />;
}

function AccentSelect({ value }: { value: string }) {
  const dark = useUi((u) => u.dark);
  const systemAccent = useUi((u) => u.appearance.systemAccent);
  const ids = [
    ...(Object.keys(BRAND_ACCENTS) as (keyof typeof BRAND_ACCENTS)[]),
    'system',
  ] as const;
  const options = ids.map((id) => ({
    value: id,
    label: id === 'system' ? t.settings.accentSystem : BRAND_ACCENTS[id].label,
    icon: <AccentSwatch color={resolveAccent(id, dark, systemAccent)} />,
  }));
  const current = ids.find((id) => id === value) ?? DEFAULT_ACCENT;
  return (
    <Select
      label={t.settings.accent}
      hint={t.settings.accentHint}
      value={current}
      options={options}
      onChange={(accent) => void saveUi({ accent })}
    />
  );
}

function AppearanceSection({ settings }: { settings: Settings }) {
  return (
    <Group>
      <Select
        label={t.settings.theme}
        value={settings.theme}
        options={THEMES}
        onChange={(theme) => void saveUi({ theme })}
      />
      <AccentSelect value={settings.accent} />
      <Checkbox
        label={t.settings.glass}
        hint={t.settings.glassHint}
        checked={settings.glassEnabled}
        onChange={(e) => void saveUi({ glassEnabled: e.target.checked })}
      />
    </Group>
  );
}

/** Configurações guardadas no cofre: carrega ao desbloquear e salva por patch. */
function useVaultSettings() {
  const { status, settings, loadSettings, updateSettings } = useVault();
  const unlocked = status?.state === 'unlocked';

  useEffect(() => {
    if (unlocked) void loadSettings();
  }, [unlocked, loadSettings]);

  const save = (patch: Partial<VaultSettings>) => {
    void updateSettings(patch).then((res) => {
      if (!res.ok) toastError(res.error);
    });
  };
  return { unlocked, settings, save };
}

function VaultSection() {
  const { unlocked, settings, save } = useVaultSettings();
  const [sub, setSub] = useState<'password' | 'erase' | null>(null);

  if (!unlocked) return <p className={s.notice}>{t.settings.vaultLocked}</p>;
  if (!settings) return null;

  return (
    <>
      <AutoLockFields saved={settings} save={save} />
      <Group title={t.settings.masterPassword}>
        <div className={s.actions}>
          <Button
            onClick={() => {
              setSub('password');
            }}
          >
            {t.settings.changePassword}
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              setSub('erase');
            }}
          >
            {t.settings.eraseVault}
          </Button>
        </div>
        <p className={s.hint}>{t.settings.eraseHint}</p>
      </Group>
      {sub === 'password' && (
        <ChangePasswordDialog
          onClose={() => {
            setSub(null);
          }}
        />
      )}
      {sub === 'erase' && (
        <EraseVaultDialog
          onClose={() => {
            setSub(null);
          }}
        />
      )}
    </>
  );
}

function AutoLockFields({
  saved,
  save,
}: {
  saved: VaultSettings;
  save: (patch: Partial<VaultSettings>) => void;
}) {
  const [autoLockOn, setAutoLockOn] = useField(saved.autoLockMinutes !== null);
  const [autoLock, setAutoLock] = useField(String(saved.autoLockMinutes ?? 15));
  const autoLockValue = intIn(autoLock, 1, 120);

  useAutosave(
    !autoLockOn || autoLockValue !== null
      ? { autoLockMinutes: autoLockOn ? autoLockValue : null }
      : null,
    { autoLockMinutes: saved.autoLockMinutes },
    save,
  );

  return (
    <Group>
      <Checkbox
        label={t.settings.autoLock}
        checked={autoLockOn}
        onChange={(e) => {
          setAutoLockOn(e.target.checked);
        }}
      />
      {autoLockOn && (
        <NumberField
          label={t.settings.autoLockMinutes}
          min={1}
          max={120}
          unit="min"
          value={autoLock}
          error={autoLockValue === null ? '1–120' : null}
          onChange={setAutoLock}
        />
      )}
    </Group>
  );
}

function ConnectionSection() {
  const { unlocked, settings, save } = useVaultSettings();
  if (!unlocked) return <p className={s.notice}>{t.settings.vaultLocked}</p>;
  if (!settings) return null;
  return <ConnectionFields saved={settings} save={save} />;
}

function ConnectionFields({
  saved,
  save,
}: {
  saved: VaultSettings;
  save: (patch: Partial<VaultSettings>) => void;
}) {
  const [timeout, setTimeoutText] = useField(String(saved.connectTimeoutSeconds));
  const [keepalive, setKeepalive] = useField(String(saved.keepaliveSeconds));
  const timeoutValue = intIn(timeout, 5, 120);
  const keepaliveValue = intIn(keepalive, 10, 300);
  const [term, setTerm] = useField(saved.term);
  const [legacyDefault, setLegacyDefault] = useField(saved.legacyDefault);

  useAutosave(
    timeoutValue !== null && keepaliveValue !== null
      ? { connectTimeoutSeconds: timeoutValue, keepaliveSeconds: keepaliveValue }
      : null,
    {
      connectTimeoutSeconds: saved.connectTimeoutSeconds,
      keepaliveSeconds: saved.keepaliveSeconds,
    },
    save,
  );

  return (
    <>
      <Group>
        <NumberField
          label={t.settings.connectTimeout}
          min={5}
          max={120}
          unit="s"
          value={timeout}
          error={timeoutValue === null ? '5–120' : null}
          onChange={setTimeoutText}
        />
        <NumberField
          label={t.settings.keepalive}
          hint={t.settings.keepaliveHint}
          min={10}
          max={300}
          step={5}
          unit="s"
          value={keepalive}
          error={keepaliveValue === null ? '10–300' : null}
          onChange={setKeepalive}
        />
      </Group>
      <Group>
        <Select
          label={t.settings.term}
          hint={t.settings.termHint}
          value={term}
          options={TERMS}
          onChange={(value) => {
            setTerm(value);
            save({ term: value });
          }}
        />
        <Checkbox
          label={t.settings.legacyDefault}
          hint={t.settings.legacyDefaultHint}
          checked={legacyDefault}
          onChange={(e) => {
            setLegacyDefault(e.target.checked);
            save({ legacyDefault: e.target.checked });
          }}
        />
      </Group>
    </>
  );
}

function ShortcutsSection() {
  return (
    <Group>
      <ShortcutsEditor />
    </Group>
  );
}

function UpdateButton() {
  const [busy, setBusy] = useState<'check' | 'install' | null>(null);
  const [latest, setLatest] = useState<string | null>(null);
  const check = async () => {
    setBusy('check');
    const res = await window.openterminal.app.checkUpdate(__APP_VERSION__);
    setBusy(null);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    if (!res.value.available) {
      toastInfo(t.settings.upToDate);
      return;
    }
    setLatest(res.value.latest);
  };
  const install = async () => {
    setBusy('install');
    const res = await window.openterminal.app.installUpdate(__APP_VERSION__);
    setBusy(null);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    if (res.value.started) toastInfo(t.settings.updateStarted);
    else if (res.value.openedPage) toastInfo(t.settings.updatePage);
    else toastInfo(t.settings.upToDate);
  };
  return (
    <div className={s.actions}>
      {latest ? (
        <>
          <p className={s.hint}>{t.settings.updateAvailable(latest)}</p>
          <Button onClick={() => void install()} disabled={busy !== null}>
            {busy === 'install' ? t.settings.installingUpdate : t.settings.installUpdate(latest)}
          </Button>
        </>
      ) : (
        <Button onClick={() => void check()} disabled={busy !== null}>
          {busy === 'check' ? t.settings.checkingUpdate : t.settings.checkUpdate}
        </Button>
      )}
    </div>
  );
}

function AboutSection() {
  return (
    <Group>
      <p className={s.appName}>{t.common.appName}</p>
      <p className={s.hint}>{t.settings.version(__APP_VERSION__)}</p>
      <UpdateButton />
      <p>{t.settings.aboutDescription}</p>
      <p className={s.hint}>{t.settings.aboutOffline}</p>
    </Group>
  );
}

export function SettingsPage() {
  const setView = useWorkspace((w) => w.setView);
  const settings = useUi((u) => u.settings);
  const [section, setSection] = useState<Section>('general');
  // No celular não há teclado físico: a seção de atalhos some.
  const mobile = useMobile();
  const sections = mobile ? SECTIONS.filter((x) => x.id !== 'shortcuts') : SECTIONS;
  const navRef = useRef<HTMLElement>(null);
  const indicator = useSlidingIndicator(navRef, section, 'y', '[aria-current="page"]');

  function close() {
    setView('terminal');
    const active = useSessions.getState().activeId;
    if (active) {
      requestAnimationFrame(() => {
        focusTerminal(active);
      });
    }
  }
  const closeRef = useRef(close);
  useEffect(() => {
    closeRef.current = close;
  });

  // Esc volta às abas, a menos que um diálogo ou lista aberta já tenha tratado a tecla.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented) return;
      if (document.querySelector('[aria-modal="true"]')) return;
      e.preventDefault();
      closeRef.current();
    };
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
    };
  }, []);

  // Foco na seção ativa ao abrir (depois que a paleta, se veio dela, devolver o foco).
  useEffect(() => {
    const timer = setTimeout(() => {
      navRef.current?.querySelector<HTMLElement>('[aria-current="page"]')?.focus();
    }, 0);
    return () => {
      clearTimeout(timer);
    };
  }, []);

  function onNavKey(e: React.KeyboardEvent) {
    const i = sections.findIndex((x) => x.id === section);
    let next: number;
    if (e.key === 'ArrowDown') next = (i + 1) % sections.length;
    else if (e.key === 'ArrowUp') next = (i - 1 + sections.length) % sections.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = sections.length - 1;
    else return;
    e.preventDefault();
    const id = sections[next]?.id ?? 'general';
    setSection(id);
    navRef.current?.querySelector<HTMLElement>(`[data-section="${id}"]`)?.focus();
  }

  return (
    <section className={s.page} aria-labelledby="settings-title">
      <header className={s.header}>
        <Button
          variant="ghost"
          iconOnly
          aria-label={t.settings.back}
          title={t.settings.back}
          onClick={close}
        >
          <BackIcon />
        </Button>
        <h1 id="settings-title" className={s.title}>
          {t.settings.title}
        </h1>
        <span className={s.autosave}>{t.settings.autosave}</span>
      </header>
      <div className={s.body}>
        <nav
          ref={navRef}
          className={s.nav}
          aria-label={t.settings.sectionsLabel}
          style={indicator ?? undefined}
          data-indicator={indicator ? '' : undefined}
          onKeyDown={onNavKey}
        >
          {sections.map(({ id, icon }) => (
            <button
              key={id}
              type="button"
              data-section={id}
              className={s.navItem}
              aria-current={id === section ? 'page' : undefined}
              tabIndex={id === section ? 0 : -1}
              onClick={() => {
                setSection(id);
              }}
            >
              {icon}
              {t.settings.sections[id]}
            </button>
          ))}
        </nav>
        <div
          className={`${s.content} ${section === 'terminal' ? s.wide : section === 'shortcuts' ? s.medium : ''}`}
          key={section}
        >
          <h2 className={s.sectionTitle}>{t.settings.sections[section]}</h2>
          {settings && section === 'general' && <GeneralSection settings={settings} />}
          {settings && section === 'appearance' && <AppearanceSection settings={settings} />}
          {section === 'terminal' && (
            <>
              {settings && <TerminalBehavior settings={settings} />}
              <GlobalTerminalSettings />
            </>
          )}
          {section === 'connection' && <ConnectionSection />}
          {section === 'vault' && <VaultSection />}
          {section === 'sync' && <SyncSection />}
          {section === 'transfer' && <TransferSection />}
          {section === 'shortcuts' && <ShortcutsSection />}
          {section === 'about' && <AboutSection />}
        </div>
      </div>
    </section>
  );
}
