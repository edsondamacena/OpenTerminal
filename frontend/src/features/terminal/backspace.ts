// Backspace configurável (como no PuTTY): o xterm manda ^? (0x7f), mas muitos equipamentos de rede
// via Telnet tratam ^? como "apagar para frente" e só apagam para trás com ^H (0x08).
import type { BackspaceMode, SessionKind } from '@shared/model/types';

const DEL = '\x7f';
const CTRL_H = '\b';

/** Converte o ^? da tecla Backspace em ^H quando o modo (ou o protocolo, no automático) pede. */
export function mapBackspace(data: string, mode: BackspaceMode, kind: SessionKind): string {
  const ctrlH = mode === 'ctrlh' || (mode === 'auto' && kind === 'telnet');
  return ctrlH && data.includes(DEL) ? data.replaceAll(DEL, CTRL_H) : data;
}
