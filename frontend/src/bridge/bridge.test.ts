import { describe, expect, it } from 'vitest';
import { toAppError } from './index';

describe('toAppError', () => {
  it('lê o JSON do erro Go', () => {
    expect(toAppError('{"code":"AUTH_FAILED","message":"A autenticação falhou."}')).toEqual({
      code: 'AUTH_FAILED',
      message: 'A autenticação falhou.',
    });
  });

  it('aceita Error com a mensagem em JSON', () => {
    expect(toAppError(new Error('{"code":"VAULT_LOCKED","message":"bloqueado"}')).code).toBe(
      'VAULT_LOCKED',
    );
  });

  it('não repassa texto fora do contrato', () => {
    const e = toAppError('dial tcp 10.0.0.1:22: senha=segredo');
    expect(e.code).toBe('INTERNAL');
    expect(e.message).not.toContain('segredo');
    expect(e.detail).not.toContain('segredo');
  });

  it('guarda a operação e o detalhe técnico (FR-174)', () => {
    const e = toAppError(
      '{"code":"SFTP_ERROR","message":"Falha no SFTP.","detail":"open /x: permission denied"}',
      'SftpService.List',
    );
    expect(e).toEqual({
      code: 'SFTP_ERROR',
      message: 'Falha no SFTP.',
      op: 'SftpService.List',
      detail: 'open /x: permission denied',
    });
  });

  it('op e detail vazios do Go não apagam a operação da chamada', () => {
    const e = toAppError(
      '{"code":"SCHEME_INVALID","message":"Inválido.","op":"","detail":""}',
      'ProfileService.ImportScheme',
    );
    expect(e).toEqual({
      code: 'SCHEME_INVALID',
      message: 'Inválido.',
      op: 'ProfileService.ImportScheme',
    });
  });

  it('rejeita código desconhecido', () => {
    expect(toAppError('{"code":"OUTRO","message":"x"}').code).toBe('INTERNAL');
  });
});
