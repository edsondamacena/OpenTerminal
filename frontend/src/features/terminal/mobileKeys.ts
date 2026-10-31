// Barra de teclas do Android: o app nativo chama `window.openterminalKeys` para mandar Esc, Tab,
// setas e para armar o Ctrl, que vale para a próxima tecla do teclado do celular.

type Writer = (data: string) => void;

declare global {
  interface Window {
    openterminalKeys?: { send: (data: string) => void; ctrl: (armed: boolean) => void };
    /** Ponte do WebView do Android; ausente no desktop. */
    OpenTerminalAndroid?: { ctrlUsed: () => void };
  }
}

let writer: Writer | null = null;
let ctrlArmed = false;

/** O terminal ativo registra por onde escrever; devolve o cancelamento. */
export function setActiveWriter(next: Writer): () => void {
  writer = next;
  return () => {
    if (writer === next) writer = null;
  };
}

/** Com o Ctrl armado, uma letra vira o caractere de controle (Ctrl+C → \x03). */
export function applyCtrl(data: string): string {
  if (!ctrlArmed || data.length !== 1) return data;
  const code = data.toUpperCase().charCodeAt(0);
  if (code < 64 || code > 95) return data;
  ctrlArmed = false;
  window.OpenTerminalAndroid?.ctrlUsed();
  return String.fromCharCode(code - 64);
}

window.openterminalKeys = {
  send: (data) => writer?.(data),
  ctrl: (armed) => {
    ctrlArmed = armed;
  },
};
