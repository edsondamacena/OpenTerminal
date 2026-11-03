import { describe, expect, it } from 'vitest';
import { mapBackspace } from './backspace';

describe('mapBackspace', () => {
  it('padrão manda ^? sem mexer', () => {
    expect(mapBackspace('\x7f', 'del')).toBe('\x7f');
  });

  it('^H troca o ^? por Ctrl+H, inclusive no Alt+Backspace', () => {
    expect(mapBackspace('\x7f', 'ctrlh')).toBe('\b');
    expect(mapBackspace('\x1b\x7f', 'ctrlh')).toBe('\x1b\b');
  });

  it('texto comum passa igual', () => {
    expect(mapBackspace('abc', 'ctrlh')).toBe('abc');
  });
});
