import type { OpenTerminalApi } from '@shared/ipc/api';

declare global {
  /** Versão do package.json, injetada pelo Vite. */
  const __APP_VERSION__: string;

  interface Window {
    readonly openterminal: OpenTerminalApi;
  }
}

export {};
