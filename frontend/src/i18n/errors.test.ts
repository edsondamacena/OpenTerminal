import { describe, expect, it } from 'vitest';
import { ERROR_CAUSES, errorDetails } from './errors';

describe('errorDetails', () => {
  it('junta código, mensagem, operação, detalhe, data e versão', () => {
    const text = errorDetails(
      {
        code: 'INTERNAL',
        message: 'Ocorreu um erro inesperado.',
        op: 'ProfileService.UpdateGlobal',
        detail: 'O método ProfileService.UpdateGlobal não existe nesta versão do app.',
      },
      new Date('2026-10-24T15:00:00Z'),
      '0.2.0',
    );
    expect(text).toBe(
      [
        'Código: INTERNAL',
        'Mensagem: Ocorreu um erro inesperado.',
        'Operação: ProfileService.UpdateGlobal',
        'Detalhe: O método ProfileService.UpdateGlobal não existe nesta versão do app.',
        'Data/hora: 2026-10-24T15:00:00.000Z',
        'Versão: 0.2.0',
      ].join('\n'),
    );
  });

  it('omite operação e detalhe ausentes', () => {
    const text = errorDetails({ code: 'AUTH_FAILED', message: 'x' }, new Date(0), '1');
    expect(text).not.toContain('Operação');
    expect(text).not.toContain('Detalhe');
  });

  it('tem causa para todo código', () => {
    for (const cause of Object.values(ERROR_CAUSES)) expect(cause.length).toBeGreaterThan(0);
  });
});
