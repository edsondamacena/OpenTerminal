import { describe, expect, it } from 'vitest';
import { arrowKeys, selectionSpan, takeLines } from './touchGestures';

describe('takeLines', () => {
  it('converte o arrasto em linhas inteiras e guarda a sobra', () => {
    expect(takeLines(45, 20)).toEqual({ lines: 2, rest: 5 });
    expect(takeLines(-45, 20)).toEqual({ lines: -2, rest: -5 });
    expect(takeLines(19, 20)).toEqual({ lines: 0, rest: 19 });
  });

  it('não rola sem altura de célula', () => {
    expect(takeLines(100, 0)).toEqual({ lines: 0, rest: 0 });
  });
});

describe('selectionSpan', () => {
  it('seleciona na mesma linha, com as pontas em qualquer ordem', () => {
    expect(selectionSpan({ col: 2, row: 5 }, { col: 6, row: 5 }, 80)).toEqual({
      col: 2,
      row: 5,
      length: 5,
    });
    expect(selectionSpan({ col: 6, row: 5 }, { col: 2, row: 5 }, 80)).toEqual({
      col: 2,
      row: 5,
      length: 5,
    });
  });

  it('atravessa linhas', () => {
    expect(selectionSpan({ col: 70, row: 1 }, { col: 9, row: 2 }, 80)).toEqual({
      col: 70,
      row: 1,
      length: 20,
    });
    expect(selectionSpan({ col: 9, row: 2 }, { col: 70, row: 1 }, 80)).toEqual({
      col: 70,
      row: 1,
      length: 20,
    });
  });
});

describe('arrowKeys', () => {
  it('manda setas no modo normal e no modo aplicação', () => {
    expect(arrowKeys(2, false)).toBe('\x1b[B\x1b[B');
    expect(arrowKeys(-1, false)).toBe('\x1b[A');
    expect(arrowKeys(-2, true)).toBe('\x1bOA\x1bOA');
    expect(arrowKeys(0, true)).toBe('');
  });
});
