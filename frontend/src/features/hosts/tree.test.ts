import { describe, expect, it } from 'vitest';
import type { Group, HostView } from '@shared/model/types';
import { buildRows, rowIndent } from './tree';

const group = (id: string, parentId: string | null = null): Group => ({
  id,
  name: id,
  parentId,
  sortOrder: 0,
});

const host = (id: string, groupId: string | null): HostView => ({
  id,
  label: id,
  address: '10.0.0.1',
  port: 22,
  username: 'u',
  authMethod: 'ask',
  legacyAlgorithms: false,
  credentialId: null,
  groupId,
  createdAt: '',
  updatedAt: '',
  credentialName: null,
});

describe('recuo da árvore (B4)', () => {
  const groups = [group('a'), group('a1', 'a'), group('b')];
  const hosts = [host('h-a1', 'a1'), host('h-a', 'a'), host('raiz-1', null), host('raiz-2', null)];

  it('hosts da raiz depois de um grupo expandido ficam no nível dos grupos da raiz', () => {
    const rows = buildRows(groups, hosts, new Set());
    const indentOf = (id: string) => {
      const row = rows.find((r) => (r.kind === 'group' ? r.group.id : r.host.id) === id);
      if (!row) throw new Error(id);
      return rowIndent(row);
    };
    expect(rows.map((r) => (r.kind === 'group' ? r.group.id : r.host.id))).toEqual([
      'a',
      'a1',
      'h-a1',
      'h-a',
      'b',
      'raiz-1',
      'raiz-2',
    ]);
    expect(indentOf('raiz-1')).toBe(indentOf('a'));
    expect(indentOf('raiz-2')).toBe(indentOf('b'));
    expect(indentOf('h-a')).toBe(indentOf('a1'));
    expect(indentOf('h-a1')).toBeGreaterThan(indentOf('a1'));
  });

  it('recolher um grupo não muda o recuo dos hosts da raiz', () => {
    const open = buildRows(groups, hosts, new Set());
    const closed = buildRows(groups, hosts, new Set(['a']));
    const root = (rows: typeof open) =>
      rows.filter((r) => r.kind === 'host' && r.host.groupId === null).map(rowIndent);
    expect(root(closed)).toEqual(root(open));
  });
});
