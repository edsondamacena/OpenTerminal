// Barra de teclas do Android: o app nativo chama `window.openterminalKeys` para mandar Esc, Tab,
// setas, colar e para armar o Ctrl, que vale para a próxima tecla do teclado do celular.

export interface ActiveTerminal {
  /** Bytes crus para a sessão (teclas especiais). */
  write: (data: string) => void;
  /** Texto colado: passa pelo xterm (colagem entre colchetes) e pela confirmação de várias linhas. */
  paste: (text: string) => void;
}

declare global {
  interface Window {
    openterminalKeys?: {
      send: (data: string) => void;
      paste: (text: string) => void;
      ctrl: (armed: boolean) => void;
    };
    /** Ponte do WebView do Android; ausente no desktop. APK antigo não tem `copy`. */
    OpenTerminalAndroid?: { ctrlUsed: () => void; copy?: (text: string) => void };
  }
}

let active: ActiveTerminal | null = null;
let ctrlArmed = false;

/** O terminal visível se registra; devolve o cancelamento. */
export function setActiveTerminal(next: ActiveTerminal): () => void {
  active = next;
  return () => {
    if (active === next) active = null;
  };
}

/** Com o Ctrl armado, uma letra vira o caractere de controle (Ctrl+C → \x03). */
export function applyCtrl(data: string): string {
  if (!ctrlArmed || data.length === 0) return data;
  // Teclado do Android (Gboard) às vezes entrega a letra junto com o texto já composto: vale a última.
  const code = data.slice(-1).toUpperCase().charCodeAt(0);
  if (code < 64 || code > 95) return data;
  ctrlArmed = false;
  window.OpenTerminalAndroid?.ctrlUsed();
  return String.fromCharCode(code - 64);
}

/**
 * Copia para a área de transferência. No Android vai pela ponte nativa: o
 * `navigator.clipboard` do WebView falha sem aviso fora de um clique.
 */
export async function copyText(text: string): Promise<boolean> {
  if (!text) return false;
  const bridge = window.OpenTerminalAndroid;
  if (bridge?.copy) {
    bridge.copy(text);
    return true;
  }
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

window.openterminalKeys = {
  send: (data) => active?.write(data),
  paste: (text) => active?.paste(text),
  ctrl: (armed) => {
    ctrlArmed = armed;
  },
};
