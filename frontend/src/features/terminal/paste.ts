// Colagem segura (FR-144): várias linhas pedem confirmação mostrando a contagem e o início.

const PREVIEW_LINES = 8;
const PREVIEW_CHARS = 300;

function normalizeBreaks(text: string): string {
  return text.replace(/\r\n?/g, '\n');
}

function countLines(text: string): number {
  const clean = normalizeBreaks(text).replace(/\n$/, '');
  return clean === '' ? 0 : clean.split('\n').length;
}

export function needsConfirm(text: string): boolean {
  return countLines(text) > 1;
}

/** Torna visíveis os caracteres de controle (ESC, BEL, NUL, Tab…), exceto a quebra de linha. */
function escapeControls(text: string): string {
  // eslint-disable-next-line no-control-regex -- casar caracteres de controle é o objetivo
  return text.replace(/[\x00-\x09\x0b-\x1f\x7f]/g, (c) =>
    c === '\t' ? '\\t' : `\\x${c.charCodeAt(0).toString(16).padStart(2, '0')}`,
  );
}

export interface PasteSummary {
  lines: number;
  preview: string;
  truncated: boolean;
}

export function describePaste(text: string): PasteSummary {
  const lines = countLines(text);
  const all = normalizeBreaks(text).replace(/\n$/, '').split('\n');
  const head = escapeControls(all.slice(0, PREVIEW_LINES).join('\n'));
  if (head.length > PREVIEW_CHARS)
    return { lines, preview: `${head.slice(0, PREVIEW_CHARS)}…`, truncated: true };
  if (all.length > PREVIEW_LINES) return { lines, preview: `${head}\n…`, truncated: true };
  return { lines, preview: head, truncated: false };
}
