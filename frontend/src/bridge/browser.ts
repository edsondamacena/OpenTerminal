// Modo navegador (Linux, internal/webui): recria `window.go.bridge` e `window.runtime` do Wails
// sobre o WebSocket do backend, para o resto do app não saber em que casca está. Sem o token
// na URL (ou na sessionStorage, depois de um F5) é o Wails, e nada aqui é instalado.
import { isLive, useSessions } from '../stores/sessions';

type Handler = (payload: unknown) => void;

interface Reply {
  id?: number;
  r?: unknown;
  e?: unknown;
  ev?: string;
  p?: unknown;
  ready?: boolean;
}

const TOKEN_KEY = 'openterminal:token';

function takeToken(): string | null {
  try {
    const match = /^#t=([0-9a-f]{64})$/.exec(location.hash);
    if (match?.[1]) {
      sessionStorage.setItem(TOKEN_KEY, match[1]);
      // Tira o token da barra de histórico; ele só vive nesta aba.
      history.replaceState(null, '', location.pathname);
      return match[1];
    }
    return sessionStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

function showDisconnected(): void {
  if (document.getElementById('ot-disconnected')) return;
  const box = document.createElement('div');
  box.id = 'ot-disconnected';
  box.textContent = 'O OpenTerminal foi encerrado. Feche esta janela e abra o app de novo.';
  box.setAttribute('role', 'alert');
  box.style.cssText =
    'position:fixed;inset:auto 16px 16px 16px;z-index:2147483647;padding:12px 16px;border-radius:8px;' +
    'background:#3a1212;color:#fff;font:14px system-ui,sans-serif';
  document.body.append(box);
}

export function isBrowserShell(): boolean {
  return document.documentElement.dataset.shell === 'browser';
}

/** Instala a ponte se o app foi aberto pelo backend no navegador. Devolve se instalou. */
export function installBrowserBridge(): boolean {
  if ('runtime' in window) return false;
  const token = takeToken();
  if (!token) return false;
  document.documentElement.dataset.shell = 'browser';

  let nextId = 1;
  let ready = false;
  let closed = false;
  let queue: string[] = [];
  const pending = new Map<
    number,
    { resolve: (v: unknown) => void; reject: (e: unknown) => void }
  >();
  const listeners = new Map<string, Set<Handler>>();

  const ws = new WebSocket(`ws://${location.host}/ws`);
  ws.onopen = () => {
    ws.send(JSON.stringify({ t: token }));
  };
  ws.onmessage = (e: MessageEvent<string>) => {
    const msg = JSON.parse(e.data) as Reply;
    if (msg.ready) {
      ready = true;
      for (const q of queue) ws.send(q);
      queue = [];
      return;
    }
    if (typeof msg.ev === 'string') {
      for (const h of listeners.get(msg.ev) ?? []) h(msg.p);
      return;
    }
    const p = msg.id === undefined ? undefined : pending.get(msg.id);
    if (!p || msg.id === undefined) return;
    pending.delete(msg.id);
    if (msg.e !== undefined) p.reject(msg.e);
    else p.resolve(msg.r);
  };
  ws.onclose = () => {
    closed = true;
    for (const p of pending.values()) p.reject('');
    pending.clear();
    showDisconnected();
  };

  const invoke = (name: string, args: unknown[]): Promise<unknown> =>
    new Promise((resolve, reject) => {
      if (closed) {
        // Como o Wails: a rejeição é o texto do erro, que toAppError converte.
        // eslint-disable-next-line @typescript-eslint/prefer-promise-reject-errors
        reject('');
        return;
      }
      const id = nextId++;
      pending.set(id, { resolve, reject });
      const data = JSON.stringify({ id, m: name, a: args });
      if (ready) ws.send(data);
      else queue.push(data);
    });

  const subscribe = (name: string, handler: Handler): (() => void) => {
    let set = listeners.get(name);
    if (!set) listeners.set(name, (set = new Set()));
    set.add(handler);
    return () => set.delete(handler);
  };

  const service = (name: string) =>
    new Proxy(
      {},
      {
        get: (_, method) =>
          typeof method === 'string'
            ? (...args: unknown[]) => invoke(`${name}.${method}`, args)
            : undefined,
      },
    );
  const bridge = new Proxy(
    {},
    { get: (_, name) => (typeof name === 'string' ? service(name) : undefined) },
  );
  Object.assign(window, {
    go: { bridge },
    runtime: {
      EventsOnMultiple: (name: string, cb: Handler) => subscribe(name, cb),
      // A janela é do navegador, com barra de título própria (TitleBar esconde os botões).
      WindowIsMaximised: () => Promise.resolve(false),
    },
  });

  // O backend pede para copiar (chave pública); quem escreve na área de transferência é a página.
  subscribe('app:clipboard', (text) => {
    if (typeof text === 'string') void navigator.clipboard.writeText(text).catch(() => undefined);
  });

  // Fechar a janela do navegador encerra as sessões; com alguma viva, o Chrome pede confirmação.
  window.addEventListener('beforeunload', (e) => {
    if (useSessions.getState().tabs.some((t) => isLive(t.state))) e.preventDefault();
  });
  return true;
}
