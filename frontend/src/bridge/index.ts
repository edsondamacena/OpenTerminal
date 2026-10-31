// Adaptador da ponte Wails (contracts/bridge.md): expõe a mesma forma `OpenTerminalApi` da 001
// sobre os serviços Go. Métodos Go devolvem `(T, error)`; o erro chega como texto JSON
// `{code, message}` e vira `{ ok: false, error }` aqui. Nenhuma exceção sai do adaptador.
import type { OpenTerminalApi, Unsubscribe } from '@shared/ipc/api';
import { ERROR_CODES, type AppError, type ErrorCode } from '@shared/ipc/errors';
import type { Result } from '@shared/ipc/result';
import { EventsOn, WindowIsMaximised } from '../../wailsjs/runtime/runtime';

type GoMethod = (...args: unknown[]) => Promise<unknown>;
type GoServices = Record<string, Record<string, GoMethod | undefined> | undefined>;

declare global {
  interface Window {
    go?: { bridge?: GoServices };
  }
}

const INTERNAL: AppError = { code: 'INTERNAL', message: 'Ocorreu um erro inesperado.' };

function isErrorCode(code: unknown): code is ErrorCode {
  return typeof code === 'string' && (ERROR_CODES as readonly string[]).includes(code);
}

/** Converte a rejeição do Wails (texto do erro Go) em AppError. */
export function toAppError(reason: unknown, op?: string): AppError {
  const text = typeof reason === 'string' ? reason : reason instanceof Error ? reason.message : '';
  const where = op ? { op } : {};
  try {
    const parsed = JSON.parse(text) as {
      code?: unknown;
      message?: unknown;
      op?: unknown;
      detail?: unknown;
    };
    if (isErrorCode(parsed.code) && typeof parsed.message === 'string') {
      return {
        code: parsed.code,
        message: parsed.message,
        ...where,
        // `op` vazio vindo do Go não apaga o da chamada, que o front sempre sabe.
        ...(typeof parsed.op === 'string' && parsed.op !== '' ? { op: parsed.op } : {}),
        ...(typeof parsed.detail === 'string' && parsed.detail !== ''
          ? { detail: parsed.detail }
          : {}),
      };
    }
  } catch {
    // Texto que não é JSON: erro fora do contrato; o texto não é repassado (pode ter segredos).
  }
  return { ...INTERNAL, ...where, detail: 'Resposta fora do contrato da ponte.' };
}

async function call<T>(service: string, method: string, ...args: unknown[]): Promise<Result<T>> {
  const op = `${service}.${method}`;
  const fn = window.go?.bridge?.[service]?.[method];
  if (!fn) {
    return {
      ok: false,
      error: { ...INTERNAL, op, detail: `O método ${op} não existe nesta versão do app.` },
    };
  }
  try {
    return { ok: true, value: (await fn(...args)) as T };
  } catch (reason) {
    return { ok: false, error: toAppError(reason, op) };
  }
}

// Um único EventsOn por nome de evento; assinantes ficam num mapa local (R10, bug 2 da 001).
const handlers = new Map<string, Set<(payload: unknown) => void>>();

export const EVENT_NAMES = [
  'vault:state',
  'term:data',
  'term:state',
  'term:hostkey',
  'term:prompt',
  'term:closed',
  'sftp:transfer',
  'sftp:conflict',
  'forwards:status',
  'profiles:changed',
  'app:appearance',
  'profiles:changed',
  'app:close-requested',
  'sync:status',
] as const;
export type EventName = (typeof EVENT_NAMES)[number];

function listenAll(): void {
  for (const name of EVENT_NAMES) {
    const set = new Set<(payload: unknown) => void>();
    handlers.set(name, set);
    EventsOn(name, (payload: unknown) => {
      for (const h of set) h(payload);
    });
  }
}

export function on(name: EventName, handler: (payload: never) => void): Unsubscribe {
  const set = handlers.get(name);
  if (!set) return () => undefined;
  const h = handler as (payload: unknown) => void;
  set.add(h);
  return () => {
    set.delete(h);
  };
}

const ev =
  (name: EventName) =>
  (handler: (payload: never) => void): Unsubscribe =>
    on(name, handler);

function createApi(): OpenTerminalApi {
  const V = 'VaultService';
  const H = 'HostService';
  const C = 'CredentialService';
  const S = 'SessionService';
  const F = 'SftpService';
  const W = 'ForwardService';
  const A = 'AppService';
  const P = 'ProfileService';
  const Y = 'SyncService';
  return {
    vault: {
      getStatus: () => call(V, 'GetStatus'),
      create: (password, confirmation) => call(V, 'Create', password, confirmation),
      unlock: (password) => call(V, 'Unlock', password),
      lock: () => call(V, 'Lock'),
      changePassword: (current, next, confirmation) =>
        call(V, 'ChangePassword', current, next, confirmation),
      erase: (confirmPhrase) => call(V, 'Erase', confirmPhrase),
      getSettings: () => call(V, 'GetSettings'),
      updateSettings: (patch) => call(V, 'UpdateSettings', patch),
      reportActivity: () => call(V, 'ReportActivity'),
      onStateChanged: ev('vault:state'),
    },
    hosts: {
      list: () => call(H, 'List'),
      create: (input) => call(H, 'Create', input),
      update: (id, input) => call(H, 'Update', id, input),
      duplicate: (id) => call(H, 'Duplicate', id),
      remove: (id) => call(H, 'Remove', id),
      move: (id, groupId) => call(H, 'Move', id, groupId),
      importTabby: () => call(H, 'ImportTabby'),
      importCsv: () => call(H, 'ImportCSV'),
      exportBackup: (password) => call(H, 'ExportBackup', password),
      importBackup: (password) => call(H, 'ImportBackup', password),
    },
    groups: {
      list: () => call(H, 'ListGroups'),
      create: (name, parentId) => call(H, 'CreateGroup', name, parentId),
      rename: (id, name) => call(H, 'RenameGroup', id, name),
      move: (id, parentId) => call(H, 'MoveGroup', id, parentId),
      remove: (id) => call(H, 'RemoveGroup', id),
    },
    credentials: {
      list: () => call(C, 'List'),
      createPassword: (name, password) => call(C, 'CreatePassword', name, password),
      pickKeyFile: () => call(C, 'PickKeyFile'),
      importKey: (name, source, passphrase) => call(C, 'ImportKey', name, source, passphrase),
      generateEd25519: (name, passphrase) => call(C, 'GenerateEd25519', name, passphrase),
      copyPublicKey: (id) => call(C, 'CopyPublicKey', id),
      rename: (id, name) => call(C, 'Rename', id, name),
      remove: (id) => call(C, 'Remove', id),
    },
    sessions: {
      // O tamanho só vai ao servidor no primeiro Resize válido (FR-113).
      openSsh: (hostId) => call(S, 'OpenSsh', hostId),
      openLocal: (shell) => call(S, 'OpenLocal', shell),
      write: (sessionId, data) => call(S, 'Write', sessionId, data),
      resize: (sessionId, size) => call(S, 'Resize', sessionId, size.cols, size.rows),
      reconnect: (sessionId) => call(S, 'Reconnect', sessionId),
      close: (sessionId) => call(S, 'Close', sessionId),
      answerHostKey: (sessionId, accept) => call(S, 'AnswerHostKey', sessionId, accept),
      answerPrompt: (sessionId, answers) => call(S, 'AnswerPrompt', sessionId, answers),
      sendKeys: (sessionId, combo) => call(S, 'SendKeys', sessionId, combo),
      activeSummary: () => call(S, 'ActiveSummary'),
      onData: ev('term:data'),
      onState: ev('term:state'),
      onHostKey: ev('term:hostkey'),
      onPrompt: ev('term:prompt'),
      onClosed: ev('term:closed'),
    },
    sftp: {
      open: (hostId) => call(F, 'Open', hostId),
      list: (sessionId, path) => call(F, 'List', sessionId, path),
      mkdir: (sessionId, path) => call(F, 'Mkdir', sessionId, path),
      rename: (sessionId, from, to) => call(F, 'Rename', sessionId, from, to),
      remove: (sessionId, path, recursive) => call(F, 'Remove', sessionId, path, recursive),
      upload: (sessionId, remoteDir) => call(F, 'Upload', sessionId, remoteDir),
      download: (sessionId, remotePaths) => call(F, 'Download', sessionId, remotePaths),
      resolveConflict: (transferId, choice) => call(F, 'ResolveConflict', transferId, choice),
      cancel: (transferId) => call(F, 'Cancel', transferId),
      onTransfer: ev('sftp:transfer'),
      onConflict: ev('sftp:conflict'),
    },
    forwards: {
      list: (hostId) => call(W, 'List', hostId),
      listActive: () => call(W, 'ListActive'),
      create: (input) => call(W, 'Create', input),
      update: (id, input) => call(W, 'Update', id, input),
      remove: (id) => call(W, 'Remove', id),
      activate: (id) => call(W, 'Activate', id),
      deactivate: (id) => call(W, 'Deactivate', id),
      onStatus: ev('forwards:status'),
    },
    profiles: {
      getGlobal: () => call(P, 'GetGlobal'),
      updateGlobal: (patch) => call(P, 'UpdateGlobal', patch),
      getForHost: (hostId) => call(P, 'GetForHost', hostId),
      setHostOverride: (hostId, profile) => call(P, 'SetHostOverride', hostId, profile),
      effective: (sessionId) => call(P, 'Effective', sessionId),
      listSchemes: () => call(P, 'ListSchemes'),
      saveScheme: (scheme) => call(P, 'SaveScheme', scheme),
      duplicateScheme: (id, name) => call(P, 'DuplicateScheme', id, name),
      removeScheme: (id) => call(P, 'RemoveScheme', id),
      importScheme: () => call(P, 'ImportScheme'),
      exportScheme: (id) => call(P, 'ExportScheme', id),
      listFonts: () => call(P, 'ListFonts'),
      onChanged: ev('profiles:changed'),
    },
    app: {
      getAppearance: () => call(A, 'GetAppearance'),
      getUiSettings: () => call(A, 'GetPreferences'),
      listLocalShells: () => call(A, 'ListLocalShells'),
      updateUiSettings: (patch) => call(A, 'UpdatePreferences', patch),
      confirmClose: (confirmed) => call(A, 'ConfirmClose', confirmed),
      windowMinimise: () => call(A, 'WindowMinimise'),
      windowToggleMaximise: () => call(A, 'WindowToggleMaximise'),
      windowClose: () => call(A, 'WindowClose'),
      openExternalUrl: (url) => call(A, 'OpenExternalUrl', url),
      checkUpdate: (current) => call(A, 'CheckUpdate', current),
      installUpdate: (current) => call(A, 'InstallUpdate', current),
      windowIsMaximised: async () => {
        try {
          return { ok: true, value: await WindowIsMaximised() };
        } catch {
          return { ok: false, error: INTERNAL };
        }
      },
      onAppearanceChanged: ev('app:appearance'),
      onCloseRequested: ev('app:close-requested'),
    },
    sync: {
      getStatus: () => call(Y, 'GetStatus'),
      connect: (password) => call(Y, 'Connect', password),
      cancelConnect: () => call(Y, 'CancelConnect'),
      disconnect: () => call(Y, 'Disconnect'),
      syncNow: () => call(Y, 'SyncNow'),
      onStatus: ev('sync:status'),
    },
  };
}

/**
 * Instala `window.openterminal` sobre o Wails e mantém uma única API para os consumidores.
 */
export function installBridge(): void {
  if ('openterminal' in window) return;
  listenAll();
  Object.defineProperty(window, 'openterminal', { value: createApi(), writable: false });
}
