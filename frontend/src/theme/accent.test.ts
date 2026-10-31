import { describe, expect, it } from 'vitest';
import { accentTokens, ensureContrast, fromOklch, resolveAccent, toOklch } from './accent';
import { BRAND } from './brand.gen';
import { contrast, parseColor } from './color';

const ratio = (a: string, b: string) => contrast(parseColor(a), parseColor(b));

describe('cor de destaque (FR-171)', () => {
  it('OKLCH vai e volta sem desvio visível', () => {
    for (const hex of ['#fd7b17', '#b04e06', '#0078d4', '#ffffff', '#111111']) {
      const back = parseColor(fromOklch(toOklch(hex)));
      const orig = parseColor(hex);
      expect(Math.abs(back.r - orig.r)).toBeLessThanOrEqual(1);
      expect(Math.abs(back.g - orig.g)).toBeLessThanOrEqual(1);
      expect(Math.abs(back.b - orig.b)).toBeLessThanOrEqual(1);
    }
  });

  it('o padrão é o laranja da marca', () => {
    expect(resolveAccent('laranja', true, null)).toBe(BRAND.laranjaOpenterminal);
    expect(resolveAccent('laranja', false, null)).toBe(BRAND.laranjaProfundo);
    expect(resolveAccent('desconhecido', true, null)).toBe(BRAND.laranjaOpenterminal);
  });

  it('sistema sem cor do Windows cai no laranja', () => {
    expect(resolveAccent('system', true, null)).toBe(BRAND.laranjaOpenterminal);
  });

  it.each([
    ['azul padrão do Windows', '#0078d4'],
    ['verde escuro', '#0b5d1e'],
    ['amarelo claro', '#ffe94d'],
    ['quase preto', '#101010'],
    ['quase branco', '#fafafa'],
  ])('cor do Windows (%s) é ajustada até 4,5:1 nos dois temas', (_label, hex) => {
    for (const dark of [true, false]) {
      const accent = ensureContrast(hex, dark);
      const surfaces = dark ? [BRAND.laca, BRAND.superficie] : [BRAND.papel, BRAND.branco];
      for (const surface of surfaces) expect(ratio(accent, surface)).toBeGreaterThanOrEqual(4.5);
    }
  });

  it('mantém o matiz ao ajustar', () => {
    const [, , h0] = toOklch('#0078d4');
    const [, , h1] = toOklch(ensureContrast('#0078d4', true));
    expect(Math.abs(h1 - h0)).toBeLessThan(6);
  });

  it.each(['laranja', 'grafite', 'system'])('texto do botão primário legível (%s)', (id) => {
    for (const dark of [true, false]) {
      const t = accentTokens(id, dark, '#0078d4');
      expect(ratio(t.accentText, t.accent)).toBeGreaterThanOrEqual(4.5);
    }
  });
});
