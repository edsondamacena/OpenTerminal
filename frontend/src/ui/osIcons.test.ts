import { describe, expect, it } from 'vitest';
import { OS_IDS } from '@shared/model/types';
import { OS_OPTIONS, hostOs, osLabel } from './osIcons';

describe('logo do sistema no host (FR-178)', () => {
  it('todo id do data-model tem rótulo e aparece uma vez no seletor', () => {
    for (const id of OS_IDS) expect(osLabel(id)).not.toBe('');
    expect(new Set(OS_OPTIONS.map((o) => o.id)).size).toBe(OS_IDS.length);
  });

  it('escolha do usuário vence; automático usa o detectado', () => {
    expect(hostOs({ osIcon: 'auto', detectedOs: 'alpine' })).toBe('alpine');
    expect(hostOs({ osIcon: 'cisco', detectedOs: 'alpine' })).toBe('cisco');
    expect(hostOs({ osIcon: 'auto', detectedOs: null })).toBeNull();
    // Cofre anterior ao FR-178: sem os campos.
    expect(hostOs({})).toBeNull();
  });
});
