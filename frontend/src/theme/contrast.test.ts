import { describe, expect, it } from 'vitest';
import { themeTokens } from './apply';
import { contrast, over, parseColor } from './color';
import { GLASS_MODES, SOLID_MODES, THEMES } from './tokens';

const WHITE = parseColor('#ffffff');
const BLACK = parseColor('#000000');
const MIN = 4.5;

describe('contraste do texto (FR-034)', () => {
  describe.each(GLASS_MODES)('%s', (mode) => {
    const t = THEMES[mode];
    const surface = parseColor(t.surfaceGlass);

    it('vidro tem opacidade mínima', () => {
      expect(surface.a).toBeGreaterThanOrEqual(mode === 'glass-dark' ? 0.72 : 0.78);
    });

    it.each([
      ['fundo branco', WHITE],
      ['fundo preto', BLACK],
    ])('texto legível sobre %s atrás da janela', (_label, behind) => {
      const composed = over(surface, behind);
      expect(contrast(parseColor(t.textPrimary), composed)).toBeGreaterThanOrEqual(MIN);
      expect(contrast(parseColor(t.textSecondary), composed)).toBeGreaterThanOrEqual(MIN);
    });
  });

  describe.each(SOLID_MODES)('%s', (mode) => {
    const t = THEMES[mode];
    it('texto legível sobre as superfícies', () => {
      for (const bg of [t.surfaceGlass, t.surfaceRaised, t.panelSolid]) {
        const surface = parseColor(bg);
        expect(contrast(parseColor(t.textPrimary), surface)).toBeGreaterThanOrEqual(MIN);
        expect(contrast(parseColor(t.textSecondary), surface)).toBeGreaterThanOrEqual(MIN);
      }
    });
  });

  it('terminal e SFTP têm fundo opaco em todos os modos com cor (FR-032)', () => {
    for (const mode of [...GLASS_MODES, ...SOLID_MODES]) {
      expect(parseColor(THEMES[mode].panelSolid).a).toBe(1);
      expect(parseColor(THEMES[mode].terminalBackground).a).toBe(1);
    }
  });

  it('terminal tem contraste de texto adequado', () => {
    for (const mode of [...GLASS_MODES, ...SOLID_MODES]) {
      const t = THEMES[mode];
      expect(
        contrast(parseColor(t.terminalForeground), parseColor(t.terminalBackground)),
      ).toBeGreaterThanOrEqual(7);
    }
  });

  describe.each(['laranja', 'grafite', 'system'])('destaque %s', (accent) => {
    it.each([...SOLID_MODES])('%s: anel, seleção e botão primário legíveis', (mode) => {
      const dark = mode.endsWith('dark');
      const t = themeTokens(
        { glass: false, highContrast: false, dark, systemAccent: '#0078d4' },
        dark ? 'dark' : 'light',
        accent,
      );
      for (const bg of [t.surfaceGlass, t.surfaceRaised]) {
        expect(contrast(parseColor(t.accent), parseColor(bg))).toBeGreaterThanOrEqual(MIN);
        // Texto sobre a seleção (destaque translúcido) continua legível.
        const selected = over(parseColor(t.selected), parseColor(bg));
        expect(contrast(parseColor(t.textPrimary), selected)).toBeGreaterThanOrEqual(MIN);
      }
      expect(contrast(parseColor(t.accentText), parseColor(t.accent))).toBeGreaterThanOrEqual(MIN);
    });
  });
});
