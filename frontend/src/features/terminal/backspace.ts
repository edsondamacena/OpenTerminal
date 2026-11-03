// Backspace configurável (como no PuTTY): o xterm manda ^? (0x7f), o padrão; alguns equipamentos
// só apagam para trás com ^H (0x08).
import type { BackspaceMode, DeleteKeyMode } from '@shared/model/types';

const DEL = '\x7f';
const CTRL_H = '\b';

/** Converte o ^? da tecla Backspace em ^H quando o modo pede. */
export function mapBackspace(data: string, mode: BackspaceMode): string {
  return mode === 'ctrlh' && data.includes(DEL) ? data.replaceAll(DEL, CTRL_H) : data;
}

// Tecla Delete: o xterm manda ESC[3~. Linux entende; OLT/switch não e imprime "~".
const VT_DELETE = '\x1b[3~';
const CTRL_D = '\x04';

/** Converte o ESC[3~ da tecla Delete conforme o modo (antes do mapBackspace). */
export function mapDelete(data: string, mode: DeleteKeyMode): string {
  if (mode === 'vt' || !data.includes(VT_DELETE)) return data;
  return data.replaceAll(VT_DELETE, mode === 'ctrld' ? CTRL_D : DEL);
}
