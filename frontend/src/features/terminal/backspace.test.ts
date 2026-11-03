import { describe, expect, it } from 'vitest';
import { mapBackspace } from './backspace';

describe('mapBackspace', () => {
  it('automático: ^H no Telnet, ^? no SSH e no shell local', () => {
    expect(mapBackspace('\x7f', 'auto', 'telnet')).toBe('\b');
    expect(mapBackspace('\x7f', 'auto', 'ssh')).toBe('\x7f');
    expect(mapBackspace('\x7f', 'auto', 'local')).toBe('\x7f');
  });

  it('modos fixos valem para qualquer protocolo', () => {
    expect(mapBackspace('\x7f', 'ctrlh', 'ssh')).toBe('\b');
    expect(mapBackspace('\x7f', 'del', 'telnet')).toBe('\x7f');
  });

  it('Alt+Backspace (ESC + ^?) e texto comum', () => {
    expect(mapBackspace('\x1b\x7f', 'ctrlh', 'ssh')).toBe('\x1b\b');
    expect(mapBackspace('abc', 'ctrlh', 'telnet')).toBe('abc');
  });
});
