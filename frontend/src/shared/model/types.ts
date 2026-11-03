// Tipos de entidades (data-model.md). Campos opcionais são representados por `null`.
import type { ErrorCode } from '../ipc/errors';

// --- Cofre -----------------------------------------------------------------

export type VaultState = 'nonexistent' | 'locked' | 'migratable' | 'unlocked' | 'unreadable';

export interface VaultStatus {
  state: VaultState;
  /** Só quando `state === "unreadable"`. */
  reason: 'corrupted' | 'unsupportedVersion' | null;
}

/** Sincronização do cofre com o Google Drive (cifrado com a senha mestra). */
export interface SyncStatus {
  connected: boolean;
  state: 'off' | 'locked' | 'idle' | 'syncing' | 'error';
  lastSync: string;
  error: string;
  /** O build tem o cliente OAuth do Google. */
  configured: boolean;
  email: string;
  loggingIn: boolean;
  /** Sobe a cada vez que a nuvem foi gravada no cofre local: a interface recarrega as listas. */
  pulls: number;
}

export interface VaultSettings {
  /** 1–120 minutos, ou `null` para desativado. */
  autoLockMinutes: number | null;
  /** 5–120 segundos. */
  connectTimeoutSeconds: number;
  /** 10–300 segundos; padrão 30. */
  keepaliveSeconds: number;
  /** Valor inicial de "algoritmos legados" em hosts novos. */
  legacyDefault: boolean;
  /** `$TERM` enviado no pedido de PTY. */
  term: TermType;
}

export type TermType = 'xterm-256color' | 'xterm' | 'vt100' | 'linux';

// --- Hosts, grupos, credenciais --------------------------------------------

export interface Group {
  id: string;
  name: string;
  parentId: string | null;
  sortOrder: number;
}

export type AuthMethod = 'password' | 'key' | 'ask' | 'agent';

export interface Host {
  id: string;
  label: string;
  address: string;
  port: number;
  username: string;
  authMethod: AuthMethod;
  credentialId: string | null;
  groupId: string | null;
  /** Aceita algoritmos fracos (equipamentos antigos, FR-121); mostrado com aviso. */
  legacyAlgorithms: boolean;
  /** Ausente em hosts antigos: SSH. Telnet só aceita senha (login automático) ou perguntar. */
  protocol?: Protocol;
  /** Sistema detectado na conexão (FR-178); só o Go grava. */
  detectedOs?: OsId | null;
  /** Ícone escolhido no formulário; `auto` usa o detectado. */
  osIcon?: OsId | 'auto';
  /** Ícone de equipamento; ausente usa o padrão do protocolo. */
  deviceIcon?: DeviceIconId;
  createdAt: string;
  updatedAt: string;
}

/** Ids de sistema do data-model (FR-178), na mesma ordem do Go. */
export const OS_IDS = [
  'ubuntu',
  'debian',
  'centos',
  'rhel',
  'fedora',
  'rocky',
  'almalinux',
  'alpine',
  'arch',
  'opensuse',
  'oracle',
  'amazon',
  'linux',
  'freebsd',
  'openbsd',
  'windows',
  'macos',
  'mikrotik',
  'cisco',
  'huawei',
  'juniper',
  'hpe',
  'fortinet',
  'openwrt',
] as const;

export type OsId = (typeof OS_IDS)[number];

/** Ícones de equipamento do host; espelha domain.DeviceIcons no Go. */
export const DEVICE_ICONS = [
  'ssh',
  'telnet',
  'router',
  'bng',
  'switch',
  'olt',
  'onu',
  'server',
  'firewall',
  'radio',
  'wifi',
  'edge',
] as const;
export type DeviceIconId = (typeof DEVICE_ICONS)[number];

export type HostInput = Omit<Host, 'id' | 'createdAt' | 'updatedAt' | 'detectedOs'>;

export interface HostView extends Host {
  credentialName: string | null;
}

export type CredentialKind = 'password' | 'key';

export type KeyType =
  'ssh-ed25519' | 'ecdsa-sha2-nistp256' | 'ecdsa-sha2-nistp384' | 'ecdsa-sha2-nistp521' | 'ssh-rsa';

/** Visão de credencial sem segredos — única forma que chega à interface. */
export interface CredentialSummary {
  id: string;
  name: string;
  kind: CredentialKind;
  createdAt: string;
  keyType: KeyType | null;
  publicKey: string | null;
  fingerprint: string | null;
  origin: 'imported' | 'generated' | null;
  usedByHostCount: number;
}

export interface KnownHostKey {
  endpoint: string;
  keyType: string;
  fingerprint: string;
  acceptedAt: string;
}

// --- Port forwarding --------------------------------------------------------

export interface ForwardRule {
  id: string;
  hostId: string;
  label: string | null;
  bindAddress: string;
  allowNonLoopback: boolean;
  bindPort: number;
  destHost: string;
  destPort: number;
}

export type ForwardRuleInput = Omit<ForwardRule, 'id'>;

export type ForwardStatusKind = 'inactive' | 'starting' | 'active' | 'error';

export interface ForwardRuleStatus {
  status: ForwardStatusKind;
  reason: ErrorCode | null;
}

export interface ActiveForward extends ForwardRuleStatus {
  ruleId: string;
  hostLabel: string;
  bindAddress: string;
  bindPort: number;
  destHost: string;
  destPort: number;
}

// --- Preferências (settings.json, sem segredos) ----------------------------

export type ThemePreference = 'system' | 'light' | 'dark';

export interface WindowBounds {
  width: number;
  height: number;
  x: number | null;
  y: number | null;
  maximized: boolean;
}

export interface Settings {
  theme: ThemePreference;
  terminalFontFamily: string;
  terminalFontSize: number;
  window: WindowBounds;
  /** Shell aberto pela nova aba (Ctrl+Shift+T). */
  defaultShell: LocalShell;
  /** Barra lateral em modo trilho. */
  sidebarCollapsed: boolean;
  /** Ids dos grupos recolhidos na árvore de hosts, lembrados entre reinícios. */
  collapsedGroups: string[];
  /** Vidro ligado pelo usuário (FR-162). */
  glassEnabled: boolean;
  /** `system` segue o Windows; os demais valores são ids da paleta da marca. */
  accent: string;
  /** Atalhos alterados pelo usuário: ação → combinação (`""` = desativado) (FR-173). */
  shortcuts?: Record<string, string>;
  /** Comportamento do terminal (FR-177). */
  copyOnSelect: boolean;
  rightClickPaste: boolean;
  bell: BellMode;
  boldBright: boolean;
  /** Realce de palavras de rede em saídas sem cor própria (equipamentos de rede). */
  highlight: boolean;
  autoReconnect: boolean;
  /** O que a tecla Backspace envia: `auto` = ^H no Telnet e ^? no resto. */
  backspace: BackspaceMode;
}

export type BellMode = 'off' | 'sound' | 'visual';

export type BackspaceMode = 'auto' | 'del' | 'ctrlh';

export interface Appearance {
  glass: boolean;
  highContrast: boolean;
  dark: boolean;
  /** Cor DWM convertida de ABGR para #rrggbb. */
  systemAccent: string | null;
}

// --- Perfis de terminal e esquemas de cores (US4, data-model) -------------------

export type CursorStyle = 'block' | 'bar' | 'underline';
export type SchemeOrigin = 'builtin' | 'imported' | 'custom';

export interface TerminalProfile {
  id: string;
  name: string;
  isGlobal: boolean;
  fontFamily: string;
  /** 8–32, padrão 14. */
  fontSize: number;
  /** 100–900. */
  fontWeight: number;
  fontWeightBold: number;
  /** 1.0–2.0, padrão 1.2. */
  lineHeight: number;
  /** -2–10 px. */
  letterSpacing: number;
  ligatures: boolean;
  cursorStyle: CursorStyle;
  cursorBlink: boolean;
  /** `#rrggbb` ou null para usar a do esquema. */
  cursorColor: string | null;
  /** 100–100.000, padrão 10.000. */
  scrollback: number;
  /** 0.5–1.0, padrão 1.0. */
  backgroundOpacity: number;
  /** 0–40 px. */
  blur: number;
  /** 0–32 px, padrão 12. */
  padding: number;
  colorSchemeId: string;
}

export interface ColorScheme {
  id: string;
  name: string;
  origin: SchemeOrigin;
  foreground: string;
  background: string;
  cursor: string;
  /** `#rrggbb` ou `#rrggbbaa`. */
  selection: string;
  /** 16 cores ANSI: 0–7 normais, 8–15 brilhantes. */
  ansi: string[];
}

export interface FontInfo {
  family: string;
  source: 'system' | 'bundled';
  ligatures: boolean;
}

export interface SchemeImportResult {
  imported: ColorScheme[];
  /** Cores ausentes no arquivo, preenchidas com as do esquema padrão. */
  completed: string[];
}

export interface ProfilesChangedEvent {
  scope: 'global' | 'host';
  hostId?: string;
}

// --- Sessões (só em memória) ------------------------------------------------

export type SessionKind = 'ssh' | 'telnet' | 'local' | 'sftp';

export type Protocol = 'ssh' | 'telnet';

/** Resumo do import de backup: só contagens. */
export interface BackupImportResult {
  hosts: number;
  groups: number;
  credentials: number;
  knownHosts: number;
  forwardRules: number;
  duplicates: number;
  invalid: number;
}

/** Resumo do import do Tabby: só contagens, nunca nomes, endereços ou senhas. */
export interface TabbyImportResult {
  hosts: number;
  telnet: number;
  groups: number;
  credentials: number;
  noPassword: number;
  duplicates: number;
  invalid: number;
}
/** Id estável de shell local: `powershell`, `pwsh`, `cmd`, `gitbash`, `wsl:<distro>` ou, fora do
 * Windows, `bash`, `zsh`, `fish` e `sh`. */
export type LocalShell = string;

export type LocalShellKind = 'powershell' | 'pwsh' | 'cmd' | 'gitbash' | 'wsl' | 'unix';

/** Shell detectado na máquina (o caminho do executável fica no Go). */
export interface LocalShellInfo {
  id: LocalShell;
  label: string;
  kind: LocalShellKind;
}
export type SendKeysCombo = 'ctrl+w' | 'ctrl+t' | 'ctrl+tab' | 'ctrl+k' | 'ctrl+b';

export type SessionState =
  | 'connecting'
  | 'awaitingHostKey'
  | 'authenticating'
  | 'awaitingPrompt'
  | 'connected'
  | 'disconnected'
  | 'error'
  | 'closed';

export interface TermSize {
  cols: number;
  rows: number;
}

export interface SessionStateEvent {
  sessionId: string;
  state: SessionState;
  /** Mensagem pt-BR para `disconnected`. */
  reason: string | null;
  /** Código de saída para `exited`. */
  exitCode: number | null;
  /** RTT do último keepalive SSH; null antes da primeira medição. */
  latencyMs: number | null;
}

export interface SessionDataEvent {
  sessionId: string;
  data: string;
}

export interface SessionClosedEvent {
  sessionId: string;
}

export interface HostKeyEvent {
  sessionId: string;
  kind: 'new' | 'changed';
  keyType: string;
  fingerprint: string;
  previousFingerprint: string | null;
}

export interface PromptEvent {
  sessionId: string;
  name: string;
  instructions: string;
  prompts: { text: string; echo: boolean }[];
}

export interface ActiveSummary {
  sessions: number;
  transfers: number;
  forwards: number;
}

// --- SFTP -------------------------------------------------------------------

export interface RemoteEntry {
  name: string;
  path: string;
  size: number;
  modifiedAt: string;
  type: 'file' | 'dir' | 'link' | 'other';
}

export type TransferState = 'queued' | 'running' | 'completed' | 'cancelled' | 'failed' | 'skipped';
export type ConflictChoice = 'replace' | 'skip' | 'cancelAll';

export interface Transfer {
  id: string;
  sessionId: string;
  direction: 'upload' | 'download';
  localPath: string;
  remotePath: string;
  bytesTotal: number;
  bytesDone: number;
  state: TransferState;
  reason: string | null;
}

/** Botão Atualizar: última Release do GitHub. */
export interface UpdateInfo {
  current: string;
  latest: string;
  available: boolean;
  notes: string;
  pageUrl: string;
  assetName: string;
}

export interface UpdateResult extends UpdateInfo {
  /** O instalador foi aberto (depois de conferir o SHA-256). */
  started: boolean;
  /** Sem instalador automático: abriu a página da Release. */
  openedPage: boolean;
}

export interface ConflictEvent {
  transferId: string;
  name: string;
  existingSize: number;
  existingModifiedAt: string;
}
