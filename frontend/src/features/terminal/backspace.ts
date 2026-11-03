// Backspace configurável (como no PuTTY): o xterm manda ^? (0x7f), o padrão; alguns equipamentos
// só apagam para trás com ^H (0x08).
import type { BackspaceMode } from '@shared/model/types';

const DEL = '\x7f';
const CTRL_H = '\b';

/** Converte o ^? da tecla Backspace em ^H quando o modo pede. */
export function mapBackspace(data: string, mode: BackspaceMode): string {
  return mode === 'ctrlh' && data.includes(DEL) ? data.replaceAll(DEL, CTRL_H) : data;
}
