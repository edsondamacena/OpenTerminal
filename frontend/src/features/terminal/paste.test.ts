import { describe, expect, it } from 'vitest';
import { describePaste, needsConfirm } from './paste';

describe('colagem (FR-144)', () => {
  it('uma linha, com ou sem quebra no fim, não pede confirmação', () => {
    expect(needsConfirm('ls -la')).toBe(false);
    expect(needsConfirm('ls -la\n')).toBe(false);
    expect(needsConfirm('ls -la\r\n')).toBe(false);
    expect(needsConfirm('')).toBe(false);
  });

  it('várias linhas pedem confirmação com a contagem', () => {
    expect(needsConfirm('a\nb')).toBe(true);
    expect(describePaste('a\nb\nc\n').lines).toBe(3);
    expect(describePaste('a\r\nb\r\nc\r\nd\r\ne').lines).toBe(5);
    expect(describePaste('a\rb').lines).toBe(2);
  });

  it('a prévia escapa caracteres de controle e corta textos longos', () => {
    const { preview } = describePaste('echo \x1b[31mvermelho\x07\tfim\nrm -rf /tmp/x\x00');
    expect(preview).toBe('echo \\x1b[31mvermelho\\x07\\tfim\nrm -rf /tmp/x\\x00');
    const long = describePaste('x'.repeat(1000));
    expect(long.preview.length).toBeLessThanOrEqual(301);
    expect(long.preview.endsWith('…')).toBe(true);
    expect(long.truncated).toBe(true);
  });

  it('a prévia mostra no máximo as 8 primeiras linhas', () => {
    const text = Array.from({ length: 20 }, (_, i) => `linha ${String(i + 1)}`).join('\n');
    const { preview, lines, truncated } = describePaste(text);
    expect(lines).toBe(20);
    expect(preview.split('\n')).toHaveLength(9);
    expect(preview.split('\n')[8]).toBe('…');
    expect(truncated).toBe(true);
  });
});
