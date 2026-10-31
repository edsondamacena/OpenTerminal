import { describe, expect, it } from 'vitest';
import { effectiveProfile } from '../../stores/profiles';
import {
  BRAND_SCHEME,
  DEFAULT_PROFILE,
  cssFontFamily,
  schemeOf,
  schemeTheme,
  withAlpha,
} from './profile';

describe('perfil de terminal (FR-152, FR-156)', () => {
  it('converte cor e opacidade em rgba', () => {
    expect(withAlpha('#050505', 0.8)).toBe('rgba(5, 5, 5, 0.8)');
    expect(withAlpha('#ff8000', 1)).toBe('rgba(255, 128, 0, 1)');
  });

  it('monta o tema do xterm a partir do esquema', () => {
    const theme = schemeTheme(BRAND_SCHEME, DEFAULT_PROFILE);
    expect(theme.background).toBe('#050505');
    expect(theme.cursor).toBe(BRAND_SCHEME.cursor);
    expect(theme.red).toBe(BRAND_SCHEME.ansi[1]);
    expect(theme.brightWhite).toBe(BRAND_SCHEME.ansi[15]);
  });

  it('opacidade abaixo de 100% vai para o fundo; cor própria do cursor vence a do esquema', () => {
    const theme = schemeTheme(BRAND_SCHEME, {
      ...DEFAULT_PROFILE,
      backgroundOpacity: 0.5,
      cursorColor: '#ff0000',
    });
    expect(theme.background).toBe('rgba(5, 5, 5, 0.5)');
    expect(theme.cursor).toBe('#ff0000');
  });

  it('família da fonte ganha aspas quando precisa e reservas monoespaçadas', () => {
    expect(cssFontFamily('Consolas')).toBe('Consolas, "Cascadia Mono", Consolas, monospace');
    expect(cssFontFamily('JetBrains Mono')).toBe(
      '"JetBrains Mono", "Cascadia Mono", Consolas, monospace',
    );
  });

  it('esquema desconhecido cai no primeiro da lista', () => {
    expect(schemeOf([BRAND_SCHEME], 'nao-existe')).toBe(BRAND_SCHEME);
  });

  it('override do host vence o global só para aquele host', () => {
    const own = { ...DEFAULT_PROFILE, id: 'h', isGlobal: false, fontSize: 20 };
    const overrides = { host1: own, host2: null };
    expect(effectiveProfile('host1', DEFAULT_PROFILE, overrides)).toBe(own);
    expect(effectiveProfile('host2', DEFAULT_PROFILE, overrides)).toBe(DEFAULT_PROFILE);
    expect(effectiveProfile(null, DEFAULT_PROFILE, overrides)).toBe(DEFAULT_PROFILE);
  });
});
