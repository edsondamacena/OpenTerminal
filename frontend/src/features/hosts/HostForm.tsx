// Criar/editar host (FR-007).
import { useEffect, useState } from 'react';
import {
  DEVICE_ICONS,
  type AuthMethod,
  type DeviceIconId,
  type HostInput,
  type HostView,
  type OsId,
  type Protocol,
} from '@shared/model/types';
import { t } from '../../i18n/pt-BR';
import { useHosts } from '../../stores/hosts';
import { useProfiles } from '../../stores/profiles';
import { useVault } from '../../stores/vault';
import { Button } from '../../ui/Button';
import { Checkbox } from '../../ui/Checkbox';
import { Dialog } from '../../ui/Dialog';
import { NumberField } from '../../ui/NumberField';
import { Select } from '../../ui/Select';
import { TextField } from '../../ui/TextField';
import { DeviceLogo } from '../../ui/deviceIcons';
import { WarningIcon } from '../../ui/icons';
import { OS_ICON_EDITABLE, OS_OPTIONS, OsLogo, osLabel } from '../../ui/osIcons';
import { toastError } from '../../ui/Toast';
import { HostTerminalDialog } from './HostTerminalDialog';
import s from './hosts.module.css';
import { groupPaths } from './tree';

const NONE = '';
const ROOT = '';

const METHODS: { value: AuthMethod; label: string }[] = [
  { value: 'password', label: t.hosts.authPassword },
  { value: 'ask', label: t.hosts.authAsk },
  { value: 'key', label: t.hosts.authKey },
  { value: 'agent', label: t.hosts.authAgent },
];
// Login e senha ficam no próprio host (a senha vai cifrada no cofre, como antes). Chave e agente
// só aparecem para hosts antigos que já usam esses métodos.
const SIMPLE: AuthMethod[] = ['password', 'ask'];

const PROTOCOLS: { value: Protocol; label: string }[] = [
  { value: 'ssh', label: 'SSH' },
  { value: 'telnet', label: 'Telnet' },
];
const DEFAULT_PORT: Record<Protocol, string> = { ssh: '22', telnet: '23' };

export interface HostFormProps {
  host: HostView | null;
  onClose: () => void;
}

export function HostForm({ host, onClose }: HostFormProps) {
  const { credentials, groups, hosts, reload } = useHosts();
  const [label, setLabel] = useState(host?.label ?? '');
  const [groupId, setGroupId] = useState(host?.groupId ?? ROOT);
  const [address, setAddress] = useState(host?.address ?? '');
  const [protocol, setProtocol] = useState<Protocol>(host?.protocol ?? 'ssh');
  const telnet = protocol === 'telnet';
  const [port, setPort] = useState(String(host?.port ?? 22));
  const [username, setUsername] = useState(host?.username ?? '');
  const [method, setMethod] = useState<AuthMethod>(host?.authMethod ?? 'password');
  const [keyId, setKeyId] = useState<string>(
    host?.authMethod === 'key' ? (host.credentialId ?? NONE) : NONE,
  );
  const [newPassword, setNewPassword] = useState('');
  // Edição de host que já tem senha: campo em branco mantém a senha salva.
  const hasSavedPassword = host?.authMethod === 'password' && Boolean(host.credentialId);
  // Host novo começa com o padrão da seção Conexão até o usuário mexer no campo.
  const legacyDefault = useVault((v) => v.settings?.legacyDefault ?? false);
  const loadVaultSettings = useVault((v) => v.loadSettings);
  const [osIcon, setOsIcon] = useState<OsId | 'auto'>(host?.osIcon ?? 'auto');
  const [deviceIcon, setDeviceIcon] = useState<DeviceIconId | 'default'>(
    host?.deviceIcon ?? 'default',
  );
  const [legacyChoice, setLegacy] = useState<boolean | null>(host?.legacyAlgorithms ?? null);
  const legacy = legacyChoice ?? legacyDefault;
  const [busy, setBusy] = useState(false);
  const [editingProfile, setEditingProfile] = useState(false);
  const hasOwnProfile = useProfiles((p) => (host ? Boolean(p.overrides[host.id]) : false));
  const loadOverride = useProfiles((p) => p.loadOverride);
  const setOverride = useProfiles((p) => p.setOverride);
  const hostId = host?.id;

  useEffect(() => {
    if (hostId) void loadOverride(hostId);
  }, [hostId, loadOverride]);

  useEffect(() => {
    if (!host) void loadVaultSettings();
  }, [host, loadVaultSettings]);

  const keyOptions = [
    { value: NONE, label: t.hosts.noCredential },
    ...credentials.filter((c) => c.kind === 'key').map((c) => ({ value: c.id, label: c.name })),
  ];
  const methodOptions = METHODS.filter(
    (m) =>
      (SIMPLE.includes(m.value) || m.value === host?.authMethod) &&
      (!telnet || m.value === 'password' || m.value === 'ask'),
  );
  const paths = groupPaths(groups);
  const groupOptions = [
    { value: ROOT, label: t.groups.root },
    ...groups
      .map((g) => ({ value: g.id, label: paths.get(g.id) ?? g.name }))
      .sort((a, b) => a.label.localeCompare(b.label, 'pt-BR')),
  ];
  const portNumber = Number(port);
  const portValid = Number.isInteger(portNumber) && portNumber >= 1 && portNumber <= 65535;
  // Telnet: há equipamento que só pede a senha.
  const userOptional = telnet;
  const valid =
    label.trim() !== '' &&
    address.trim() !== '' &&
    (userOptional || username.trim() !== '') &&
    portValid &&
    (method !== 'password' || newPassword !== '' || hasSavedPassword) &&
    (method !== 'key' || keyId !== NONE);

  async function submit() {
    if (!valid) return;
    setBusy(true);
    let credId: string | null = null;
    if (method === 'key') credId = keyId;
    else if (method === 'password') {
      if (newPassword === '') credId = host?.credentialId ?? null;
      else {
        const res = await window.openterminal.credentials.createPassword(label.trim(), newPassword);
        if (!res.ok) {
          setBusy(false);
          toastError(res.error);
          return;
        }
        credId = res.value.id;
      }
    }
    const input: HostInput = {
      label: label.trim(),
      address: address.trim(),
      port: portNumber,
      username: username.trim(),
      authMethod: method,
      credentialId: credId,
      groupId: groupId === ROOT ? null : groupId,
      legacyAlgorithms: telnet ? false : legacy,
      protocol,
      ...(deviceIcon === 'default' ? {} : { deviceIcon }),
      ...(OS_ICON_EDITABLE ? { osIcon } : {}),
    };
    const res = host
      ? await window.openterminal.hosts.update(host.id, input)
      : await window.openterminal.hosts.create(input);
    setBusy(false);
    if (!res.ok) {
      toastError(res.error);
      return;
    }
    // Senha trocada: a antiga sai do cofre se nenhum outro host a usa.
    const old = host?.credentialId;
    if (old && old !== credId && !hosts.some((h) => h.id !== host.id && h.credentialId === old)) {
      const del = await window.openterminal.credentials.remove(old);
      if (!del.ok) toastError(del.error);
    }
    await reload();
    onClose();
  }

  return (
    <Dialog
      title={host ? t.hosts.edit : t.hosts.add}
      onClose={onClose}
      onSubmit={() => void submit()}
      width={520}
      actions={
        <>
          <Button onClick={onClose}>{t.common.cancel}</Button>
          <Button type="submit" variant="primary" disabled={busy || !valid}>
            {t.common.save}
          </Button>
        </>
      }
    >
      <div className={s.form}>
        <TextField
          label={t.hosts.label}
          autoFocus
          maxLength={100}
          value={label}
          onChange={(e) => {
            setLabel(e.target.value);
          }}
        />
        <Select
          label={t.hosts.group}
          value={groupId}
          options={groupOptions}
          onChange={setGroupId}
        />
        {OS_ICON_EDITABLE && (
          <Select
            label={t.hosts.osIcon}
            value={osIcon}
            options={[
              {
                value: 'auto',
                label: host?.detectedOs
                  ? t.hosts.osIconAutoDetected(osLabel(host.detectedOs))
                  : t.hosts.osIconAuto,
              },
              ...OS_OPTIONS.map((os) => ({
                value: os.id,
                label: os.label,
                icon: <OsLogo id={os.id} />,
              })),
            ]}
            onChange={setOsIcon}
          />
        )}
        <Select
          label={t.hosts.deviceIcon}
          value={deviceIcon}
          options={[
            {
              value: 'default' as const,
              label: t.hosts.deviceIconDefault(telnet ? 'Telnet' : 'SSH'),
              icon: <DeviceLogo id={telnet ? 'telnet' : 'ssh'} />,
            },
            ...DEVICE_ICONS.map((id) => ({
              value: id,
              label: t.hosts.deviceIcons[id],
              icon: <DeviceLogo id={id} />,
            })),
          ]}
          onChange={setDeviceIcon}
        />
        <Select
          label={t.hosts.protocol}
          value={protocol}
          options={PROTOCOLS}
          onChange={(v) => {
            // Porta ainda no padrão do protocolo anterior acompanha a troca.
            if (port === DEFAULT_PORT[protocol]) setPort(DEFAULT_PORT[v]);
            if (v === 'telnet' && method !== 'password' && method !== 'ask') setMethod('password');
            setProtocol(v);
          }}
        />
        {telnet && (
          <div className={s.legacyWarning} role="note">
            <WarningIcon style={{ flex: 'none' }} />
            <p>{t.hosts.telnetHint}</p>
          </div>
        )}
        <div className={s.row}>
          <TextField
            label={t.hosts.address}
            hint={t.hosts.addressHint}
            value={address}
            onChange={(e) => {
              setAddress(e.target.value);
            }}
          />
          <NumberField
            label={t.hosts.port}
            min={1}
            max={65535}
            value={port}
            error={portValid ? null : '1–65535'}
            onChange={setPort}
          />
        </div>
        <TextField
          label={t.hosts.username}
          value={username}
          onChange={(e) => {
            setUsername(e.target.value);
          }}
        />
        <Select
          label={t.hosts.authMethod}
          value={method}
          options={methodOptions}
          onChange={(v) => {
            setMethod(v);
          }}
        />
        {method === 'key' && (
          <Select label={t.hosts.credential} value={keyId} options={keyOptions} onChange={setKeyId} />
        )}
        {method === 'agent' && <p className={s.hint}>{t.hosts.authAgentHint}</p>}
        {method === 'password' && (
          <TextField
            label={t.hosts.password}
            hint={hasSavedPassword ? t.hosts.passwordKeep : undefined}
            placeholder={hasSavedPassword ? '••••••••' : undefined}
            autoComplete="new-password"
            secret
            value={newPassword}
            onChange={(e) => {
              setNewPassword(e.target.value);
            }}
          />
        )}
        {!telnet && (
          <Checkbox
            label={t.hosts.legacy}
            checked={legacy}
            onChange={(e) => {
              setLegacy(e.target.checked);
            }}
          />
        )}
        {!telnet && legacy && (
          <div className={s.legacyWarning} role="note">
            <WarningIcon style={{ flex: 'none' }} />
            <p>{t.hosts.legacyHint}</p>
          </div>
        )}
        {host && (
          <div className={s.terminalProfile} role="group" aria-label={t.hosts.terminalProfile}>
            <p>{hasOwnProfile ? t.hosts.terminalProfileOwn : t.hosts.terminalProfileGlobal}</p>
            <Button
              onClick={() => {
                setEditingProfile(true);
              }}
            >
              {hasOwnProfile ? t.hosts.terminalProfileEdit : t.hosts.terminalProfileCustomize}
            </Button>
            {hasOwnProfile && (
              <Button variant="ghost" onClick={() => void setOverride(host.id, null)}>
                {t.hosts.terminalProfileReset}
              </Button>
            )}
          </div>
        )}
      </div>
      {host && editingProfile && (
        <HostTerminalDialog
          hostId={host.id}
          hostLabel={host.label}
          onClose={() => {
            setEditingProfile(false);
          }}
        />
      )}
    </Dialog>
  );
}
