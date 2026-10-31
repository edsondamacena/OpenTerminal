import { describe, expect, it } from 'vitest';
import { backoffMs, MAX_ATTEMPTS } from './autoReconnect';

describe('backoffMs', () => {
  it('dobra a espera a cada tentativa até 30 s', () => {
    expect(Array.from({ length: MAX_ATTEMPTS }, (_, i) => backoffMs(i + 1))).toEqual([
      1_000, 2_000, 4_000, 8_000, 16_000, 30_000,
    ]);
  });

  it('nunca passa de 30 s nem fica abaixo de 1 s', () => {
    expect(backoffMs(20)).toBe(30_000);
    expect(backoffMs(0)).toBe(1_000);
  });
});
