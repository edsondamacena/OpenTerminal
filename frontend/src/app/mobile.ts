// Modo celular: no app Android ou em tela estreita, a lista de hosts e o console se revezam em tela
// cheia em vez de dividir a largura (o console espremido ao lado da lista não dá para ler).
import { useSyncExternalStore } from 'react';

const query = window.matchMedia('(max-width: 700px)');

function subscribe(onChange: () => void) {
  query.addEventListener('change', onChange);
  return () => {
    query.removeEventListener('change', onChange);
  };
}

export function isMobile(): boolean {
  return window.OpenTerminalAndroid !== undefined || query.matches;
}

export function useMobile(): boolean {
  return useSyncExternalStore(subscribe, isMobile);
}
