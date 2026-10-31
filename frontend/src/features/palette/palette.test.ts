import { describe, expect, it } from 'vitest';
import { filterCommands, type Command } from './palette';

const cmd = (
  id: string,
  label: string,
  hint = '',
  group: Command['group'] = 'action',
): Command => ({
  id,
  label,
  hint,
  group,
  run: () => undefined,
});

const items: Command[] = [
  cmd('settings', 'Abrir configurações'),
  cmd('lock', 'Bloquear o cofre'),
  cmd('local', 'Novo terminal PowerShell'),
  cmd('h1', 'Olt São José', 'admin@10.0.0.1', 'host'),
  cmd('h2', 'core-router', 'noc@192.168.1.1', 'host'),
];

const ids = (q: string) => filterCommands(items, q).map((c) => c.id);

describe('filtro da paleta (FR-141)', () => {
  it('sem texto mostra tudo, na ordem', () => {
    expect(ids('')).toEqual(['settings', 'lock', 'local', 'h1', 'h2']);
  });

  it('substring sem caixa nem acento em ações e hosts', () => {
    expect(ids('configuracoes')).toEqual(['settings']);
    expect(ids('SAO JOSE')).toEqual(['h1']);
    expect(ids('cofre')).toEqual(['lock']);
  });

  it('procura também no endereço e usuário do host', () => {
    expect(ids('192.168')).toEqual(['h2']);
    expect(ids('admin@')).toEqual(['h1']);
  });

  it('quem começa com o texto vem antes de quem só o contém', () => {
    expect(ids('o')).toEqual(['h1', 'settings', 'lock', 'local', 'h2']);
  });

  it('nada encontrado devolve lista vazia', () => {
    expect(ids('zzz')).toEqual([]);
  });
});
