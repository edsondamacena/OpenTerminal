// Toque com um dedo no terminal (celular). O xterm 6 não rola com o dedo, e no Android segurar o
// dedo abria o menu de contexto, que colava a área de transferência. Aqui:
//   arrastar          → rola o histórico (em vim/less/htop manda setas, como o Termux)
//   segurar e arrastar → seleciona; ao soltar copia
// Dois dedos ficam com a pinça de zoom, tratada em TerminalView.
import type { Terminal } from '@xterm/xterm';

const LONG_PRESS_MS = 450;
const MOVE_SLOP_PX = 10;

export interface Cell {
  col: number;
  /** Linha no buffer inteiro (histórico incluído), não na tela. */
  row: number;
}

/** Converte o deslocamento acumulado do dedo em linhas inteiras; devolve as linhas e a sobra. */
export function takeLines(
  accumulated: number,
  cellHeight: number,
): { lines: number; rest: number } {
  if (cellHeight <= 0) return { lines: 0, rest: 0 };
  const lines = Math.trunc(accumulated / cellHeight);
  return { lines, rest: accumulated - lines * cellHeight };
}

/** Início e tamanho para `term.select`, com as pontas em qualquer ordem. */
export function selectionSpan(
  a: Cell,
  b: Cell,
  cols: number,
): { col: number; row: number; length: number } {
  const [first, last] = a.row < b.row || (a.row === b.row && a.col <= b.col) ? [a, b] : [b, a];
  return {
    col: first.col,
    row: first.row,
    length: (last.row - first.row) * cols + (last.col - first.col) + 1,
  };
}

/** Setas para rolar programas de tela cheia; positivo = conteúdo mais abaixo. */
export function arrowKeys(lines: number, applicationCursor: boolean): string {
  const key = applicationCursor
    ? lines > 0
      ? '\x1bOB'
      : '\x1bOA'
    : lines > 0
      ? '\x1b[B'
      : '\x1b[A';
  return key.repeat(Math.abs(lines));
}

export interface GestureOptions {
  term: Terminal;
  container: HTMLElement;
  /** Bytes para a sessão (setas em programas de tela cheia). */
  write: (data: string) => void;
  /** Chamado com o texto selecionado quando o dedo sai. */
  onCopy: (text: string) => void;
}

/** Liga os gestos de um dedo no contêiner do xterm; devolve o desligamento. */
export function attachTouchGestures({
  term,
  container,
  write,
  onCopy,
}: GestureOptions): () => void {
  let mode: 'pending' | 'scroll' | 'select' | null = null;
  let startX = 0;
  let startY = 0;
  let lastY = 0;
  let accumulated = 0;
  let anchor: Cell | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const cancelTimer = () => {
    if (timer) clearTimeout(timer);
    timer = null;
  };

  const metrics = () => {
    const screen = container.querySelector<HTMLElement>('.xterm-screen') ?? container;
    const rect = screen.getBoundingClientRect();
    return {
      rect,
      cellWidth: term.cols > 0 ? rect.width / term.cols : 0,
      cellHeight: term.rows > 0 ? rect.height / term.rows : 0,
    };
  };

  const cellAt = (x: number, y: number): Cell => {
    const { rect, cellWidth, cellHeight } = metrics();
    const col = cellWidth > 0 ? Math.floor((x - rect.left) / cellWidth) : 0;
    const line = cellHeight > 0 ? Math.floor((y - rect.top) / cellHeight) : 0;
    return {
      col: Math.min(Math.max(col, 0), term.cols - 1),
      row: term.buffer.active.viewportY + Math.min(Math.max(line, 0), term.rows - 1),
    };
  };

  const selectTo = (cell: Cell) => {
    if (!anchor) return;
    const span = selectionSpan(anchor, cell, term.cols);
    term.select(span.col, span.row, span.length);
  };

  const scrollBy = (lines: number) => {
    if (lines === 0) return;
    if (term.buffer.active.type === 'alternate') {
      write(arrowKeys(lines, term.modes.applicationCursorKeysMode));
    } else {
      term.scrollLines(lines);
    }
  };

  const onStart = (e: TouchEvent) => {
    cancelTimer();
    const touch = e.touches[0];
    if (e.touches.length !== 1 || !touch) {
      mode = null;
      return;
    }
    mode = 'pending';
    startX = touch.clientX;
    startY = lastY = touch.clientY;
    accumulated = 0;
    timer = setTimeout(() => {
      if (mode !== 'pending') return;
      mode = 'select';
      anchor = cellAt(startX, startY);
      selectTo(anchor);
      if ('vibrate' in navigator) navigator.vibrate(15);
    }, LONG_PRESS_MS);
  };

  const onMove = (e: TouchEvent) => {
    const touch = e.touches[0];
    if (!mode || e.touches.length !== 1 || !touch) return;
    if (mode === 'pending') {
      if (Math.hypot(touch.clientX - startX, touch.clientY - startY) < MOVE_SLOP_PX) return;
      cancelTimer();
      mode = 'scroll';
    }
    e.preventDefault();
    e.stopPropagation();
    if (mode === 'select') {
      selectTo(cellAt(touch.clientX, touch.clientY));
      return;
    }
    // Dedo para cima mostra o que vem depois (rolagem natural do celular).
    accumulated += lastY - touch.clientY;
    lastY = touch.clientY;
    const { lines, rest } = takeLines(accumulated, metrics().cellHeight);
    accumulated = rest;
    scrollBy(lines);
  };

  const onEnd = (e: TouchEvent) => {
    cancelTimer();
    if (mode === 'select') {
      e.preventDefault(); // sem o clique sintético, que limparia a seleção
      const text = term.getSelection();
      if (text) onCopy(text);
    }
    if (e.touches.length === 0) mode = null;
    anchor = null;
  };

  container.addEventListener('touchstart', onStart, { capture: true, passive: true });
  container.addEventListener('touchmove', onMove, { capture: true, passive: false });
  container.addEventListener('touchend', onEnd, { capture: true, passive: false });
  container.addEventListener('touchcancel', onEnd, { capture: true, passive: false });
  return () => {
    cancelTimer();
    container.removeEventListener('touchstart', onStart, true);
    container.removeEventListener('touchmove', onMove, true);
    container.removeEventListener('touchend', onEnd, true);
    container.removeEventListener('touchcancel', onEnd, true);
  };
}
