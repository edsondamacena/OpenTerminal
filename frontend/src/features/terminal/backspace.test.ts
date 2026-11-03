import { describe, expect, it } from 'vitest';
import { mapBackspace, mapDelete } from './backspace';

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

describe('mapDelete', () => {
  it('padrão: Delete apaga para trás e segue o ^H do Backspace', () => {
    expect(mapBackspace(mapDelete('\x1b[3~', 'backspace'), 'ctrlh')).toBe('\b');
    expect(mapBackspace(mapDelete('\x1b[3~', 'backspace'), 'del')).toBe('\x7f');
  });
  it('^D e ESC[3~', () => {
    expect(mapDelete('\x1b[3~', 'ctrld')).toBe('\x04');
    expect(mapDelete('\x1b[3~', 'vt')).toBe('\x1b[3~');
  });
  it('não mexe em outras sequências', () => {
    expect(mapDelete('\x1b[A~', 'backspace')).toBe('\x1b[A~');
  });
});
