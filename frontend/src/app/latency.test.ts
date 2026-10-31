import { describe, expect, it } from 'vitest';
import { latencyLevel } from './latency';

describe('faixas de latência (FR-172)', () => {
  it('classifica nos limites', () => {
    expect(latencyLevel(1)).toBe('good');
    expect(latencyLevel(150)).toBe('good');
    expect(latencyLevel(151)).toBe('slow');
    expect(latencyLevel(400)).toBe('slow');
    expect(latencyLevel(401)).toBe('bad');
  });
});
