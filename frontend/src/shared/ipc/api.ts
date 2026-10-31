// Contrato da ponte `window.openterminal` (contracts/ipc-bridge.md).
import type {
  ActiveForward,
  ActiveSummary,
  Appearance,
  ColorScheme,
  ConflictChoice,
  ConflictEvent,
  CredentialSummary,
  ForwardRule,
  ForwardRuleInput,
  FontInfo,
  ForwardRuleStatus,
  Group,
  HostInput,
  TabbyImportResult,
  BackupImportResult,
  HostKeyEvent,
  HostView,
  LocalShell,
  LocalShellInfo,
  SendKeysCombo,
  ProfilesChangedEvent,
  PromptEvent,
  RemoteEntry,
  SchemeImportResult,
  SessionDataEvent,
  SessionClosedEvent,
  SessionStateEvent,
  Settings,
  TerminalProfile,
  TermSize,
  Transfer,
  VaultSettings,
  VaultStatus,
  SyncStatus,
  UpdateInfo,
  UpdateResult,
} from '../model/types';
import type { Result } from './result';

type R<T> = Promise<Result<T>>;
export type Unsubscribe = () => void;
type On<T> = (handler: (payload: T) => void) => Unsubscribe;

export type KeySource = { filePath: string } | { pem: string };
/** Atualização parcial; campos ausentes ficam como estão (formato de saída do zod `.partial()`). */
export type Patch<T> = { [P in keyof T]?: T[P] | undefined };

export interface OpenTerminalApi {
  vault: {
    getStatus: () => R<VaultStatus>;
    create: (password: string, confirmation: string) => R<void>;
    unlock: (password: string) => R<void>;
    lock: () => R<void>;
    changePassword: (current: string, next: string, confirmation: string) => R<void>;
    erase: (confirmPhrase: string) => R<void>;
    getSettings: () => R<VaultSettings>;
    updateSettings: (patch: Patch<VaultSettings>) => R<VaultSettings>;
    reportActivity: () => R<void>;
    onStateChanged: On<VaultStatus>;
  };
  hosts: {
    list: () => R<HostView[]>;
    create: (input: HostInput) => R<HostView>;
    update: (id: string, input: HostInput) => R<HostView>;
    duplicate: (id: string) => R<HostView>;
    remove: (id: string) => R<void>;
    move: (id: string, groupId: string | null) => R<void>;
    importTabby: () => R<TabbyImportResult>;
    importCsv: () => R<TabbyImportResult>;
    /** Caminho salvo, ou '' se o usuário cancelou a escolha. */
    exportBackup: (password: string) => R<string>;
    importBackup: (password: string) => R<BackupImportResult>;
  };
  groups: {
    list: () => R<Group[]>;
    create: (name: string, parentId: string | null) => R<Group>;
    rename: (id: string, name: string) => R<Group>;
    move: (id: string, parentId: string | null) => R<void>;
    remove: (id: string) => R<void>;
  };
  credentials: {
    list: () => R<CredentialSummary[]>;
    createPassword: (name: string, password: string) => R<CredentialSummary>;
    pickKeyFile: () => R<{ filePath: string } | null>;
    importKey: (name: string, source: KeySource, passphrase: string | null) => R<CredentialSummary>;
    generateEd25519(name: string, passphrase: string | null): R<CredentialSummary>;
    copyPublicKey: (id: string) => R<void>;
    rename: (id: string, name: string) => R<CredentialSummary>;
    remove: (id: string) => R<void>;
  };
  sessions: {
    openSsh: (hostId: string) => R<{ sessionId: string }>;
    openLocal: (shell: LocalShell) => R<{ sessionId: string }>;
    write: (sessionId: string, data: string) => R<void>;
    resize: (sessionId: string, size: TermSize) => R<void>;
    reconnect: (sessionId: string) => R<void>;
    close: (sessionId: string) => R<void>;
    answerHostKey: (sessionId: string, accept: boolean) => R<void>;
    answerPrompt: (sessionId: string, answers: string[] | null) => R<void>;
    /** Combinação que o app captura como atalho, enviada ao shell (`ctrl+w`, `ctrl+t`, `ctrl+tab`, `ctrl+k`). */
    sendKeys: (sessionId: string, combo: SendKeysCombo) => R<void>;
    activeSummary: () => R<ActiveSummary>;
    onData: On<SessionDataEvent>;
    onState: On<SessionStateEvent>;
    onHostKey: On<HostKeyEvent>;
    onPrompt: On<PromptEvent>;
    onClosed: On<SessionClosedEvent>;
  };
  sftp: {
    open: (hostId: string) => R<{ sessionId: string; homePath: string }>;
    list: (sessionId: string, path: string) => R<RemoteEntry[]>;
    mkdir: (sessionId: string, path: string) => R<void>;
    rename: (sessionId: string, from: string, to: string) => R<void>;
    remove: (sessionId: string, path: string, recursive: boolean) => R<void>;
    upload: (sessionId: string, remoteDir: string) => R<{ transferIds: string[] }>;
    download: (sessionId: string, remotePaths: string[]) => R<{ transferIds: string[] }>;
    resolveConflict: (transferId: string, choice: ConflictChoice) => R<void>;
    cancel: (transferId: string) => R<void>;
    onTransfer: On<Transfer>;
    onConflict: On<ConflictEvent>;
  };
  forwards: {
    list: (hostId: string) => R<(ForwardRule & ForwardRuleStatus)[]>;
    listActive: () => R<ActiveForward[]>;
    create: (input: ForwardRuleInput) => R<ForwardRule>;
    update: (id: string, input: ForwardRuleInput) => R<ForwardRule>;
    remove: (id: string) => R<void>;
    activate: (id: string) => R<void>;
    deactivate: (id: string) => R<void>;
    onStatus: On<{ ruleId: string } & ForwardRuleStatus>;
  };
  /** Perfis de terminal e esquemas de cores (contracts/bridge.md, ProfileService). */
  profiles: {
    getGlobal: () => R<TerminalProfile>;
    updateGlobal: (patch: Partial<TerminalProfile>) => R<TerminalProfile>;
    getForHost: (hostId: string) => R<TerminalProfile | null>;
    setHostOverride: (hostId: string, profile: TerminalProfile | null) => R<TerminalProfile | null>;
    effective: (sessionId: string) => R<TerminalProfile>;
    listSchemes: () => R<ColorScheme[]>;
    saveScheme: (scheme: ColorScheme) => R<ColorScheme>;
    duplicateScheme: (id: string, name: string) => R<ColorScheme>;
    removeScheme: (id: string) => R<void>;
    /** Diálogo nativo; null quando cancelado. */
    importScheme: () => R<SchemeImportResult | null>;
    exportScheme: (id: string) => R<void>;
    listFonts: () => R<FontInfo[]>;
    onChanged: On<ProfilesChangedEvent>;
  };
  app: {
    getAppearance: () => R<Appearance>;
    getUiSettings: () => R<Settings>;
    /** Shells instalados, em ordem estável (FR-176). */
    listLocalShells: () => R<LocalShellInfo[]>;
    updateUiSettings: (patch: Patch<Omit<Settings, 'window'>>) => R<Settings>;
    confirmClose: (confirmed: boolean) => R<void>;
    /** Botões da barra de título própria (FR-133); fechar passa pela confirmação de FR-028. */
    windowMinimise: () => R<void>;
    windowToggleMaximise: () => R<void>;
    windowClose: () => R<void>;
    windowIsMaximised: () => R<boolean>;
    /** Só `http`/`https`; abre no navegador padrão (FR-145). */
    openExternalUrl: (url: string) => R<void>;
    /** Procura versão nova nas Releases do GitHub. */
    checkUpdate: (current: string) => R<UpdateInfo>;
    /** Baixa, confere o SHA-256 e abre o instalador. */
    installUpdate: (current: string) => R<UpdateResult>;
    onAppearanceChanged: On<Appearance>;
    onCloseRequested: On<ActiveSummary>;
  };
  sync: {
    getStatus: () => R<SyncStatus>;
    /** Confere a senha mestra, abre o login Google no navegador e faz a primeira sincronização. */
    connect: (password: string) => R<SyncStatus>;
    cancelConnect: () => R<void>;
    disconnect: () => R<SyncStatus>;
    syncNow: () => R<SyncStatus>;
    onStatus: On<SyncStatus>;
  };
}

export type ApiModule = keyof OpenTerminalApi;

type EventKeys<M> = { [K in keyof M]: K extends `on${string}` ? K : never }[keyof M];
type InvokeKeys<M> = Exclude<keyof M, EventKeys<M>>;

/** Lista de funções e eventos por módulo; o preload gera a API a partir dela. */
export const API_SHAPE = {
  vault: {
    invoke: [
      'getStatus',
      'create',
      'unlock',
      'lock',
      'changePassword',
      'erase',
      'getSettings',
      'updateSettings',
      'reportActivity',
    ],
    events: ['onStateChanged'],
  },
  hosts: {
    invoke: [
      'list',
      'create',
      'update',
      'duplicate',
      'remove',
      'move',
      'importTabby',
      'importCsv',
      'exportBackup',
      'importBackup',
    ],
    events: [],
  },
  groups: { invoke: ['list', 'create', 'rename', 'move', 'remove'], events: [] },
  credentials: {
    invoke: [
      'list',
      'createPassword',
      'pickKeyFile',
      'importKey',
      'generateEd25519',
      'copyPublicKey',
      'rename',
      'remove',
    ],
    events: [],
  },
  sessions: {
    invoke: [
      'openSsh',
      'openLocal',
      'write',
      'resize',
      'reconnect',
      'close',
      'answerHostKey',
      'answerPrompt',
      'sendKeys',
      'activeSummary',
    ],
    events: ['onData', 'onState', 'onHostKey', 'onPrompt', 'onClosed'],
  },
  sftp: {
    invoke: [
      'open',
      'list',
      'mkdir',
      'rename',
      'remove',
      'upload',
      'download',
      'resolveConflict',
      'cancel',
    ],
    events: ['onTransfer', 'onConflict'],
  },
  forwards: {
    invoke: ['list', 'listActive', 'create', 'update', 'remove', 'activate', 'deactivate'],
    events: ['onStatus'],
  },
  profiles: {
    invoke: [
      'getGlobal',
      'updateGlobal',
      'getForHost',
      'setHostOverride',
      'effective',
      'listSchemes',
      'saveScheme',
      'duplicateScheme',
      'removeScheme',
      'importScheme',
      'exportScheme',
      'listFonts',
    ],
    events: ['onChanged'],
  },
  app: {
    invoke: [
      'getAppearance',
      'getUiSettings',
      'listLocalShells',
      'updateUiSettings',
      'confirmClose',
      'windowMinimise',
      'windowToggleMaximise',
      'windowClose',
      'windowIsMaximised',
      'openExternalUrl',
      'checkUpdate',
      'installUpdate',
    ],
    events: ['onAppearanceChanged', 'onCloseRequested'],
  },
  sync: {
    invoke: ['getStatus', 'connect', 'cancelConnect', 'disconnect', 'syncNow'],
    events: ['onStatus'],
  },
} as const satisfies {
  [M in ApiModule]: {
    invoke: readonly InvokeKeys<OpenTerminalApi[M]>[];
    events: readonly EventKeys<OpenTerminalApi[M]>[];
  };
};

/** Nome do canal IPC: `"<módulo>:<função>"`. */
export function channelOf<M extends ApiModule>(module: M, name: keyof OpenTerminalApi[M]): string {
  return `${module}:${String(name)}`;
}

/** Argumentos e retorno de uma função de invoke, para tipar os handlers do principal. */
export type InvokeArgs<
  M extends ApiModule,
  K extends keyof OpenTerminalApi[M],
> = OpenTerminalApi[M][K] extends (...args: infer A) => unknown ? A : never;

export type InvokeValue<
  M extends ApiModule,
  K extends keyof OpenTerminalApi[M],
> = OpenTerminalApi[M][K] extends (...args: never[]) => Promise<Result<infer T>> ? T : never;

export type EventPayload<M extends ApiModule, K extends keyof OpenTerminalApi[M]> =
  OpenTerminalApi[M][K] extends On<infer P> ? P : never;
