// Árvore de grupos e hosts achatada em linhas para a barra lateral (US3).
import type { Group, HostView } from '@shared/model/types';

export type TreeRow =
  | { kind: 'group'; group: Group; depth: number; expanded: boolean; hostCount: number }
  | { kind: 'host'; host: HostView; depth: number };

/** Recuo à esquerda (px) de uma linha: grupos e hosts do mesmo nível alinham (B4). */
export const INDENT_BASE = 8;
export const INDENT_STEP = 16;

export function rowIndent(row: TreeRow): number {
  return INDENT_BASE + row.depth * INDENT_STEP;
}

const byLabel = (a: HostView, b: HostView) => a.label.localeCompare(b.label, 'pt-BR');
const byOrder = (a: Group, b: Group) =>
  a.sortOrder - b.sortOrder || a.name.localeCompare(b.name, 'pt-BR');

/** Linhas visíveis: em cada nível, subgrupos (por `sortOrder`) e depois hosts (por rótulo). */
export function buildRows(
  groups: readonly Group[],
  hosts: readonly HostView[],
  collapsed: ReadonlySet<string>,
): TreeRow[] {
  const known = new Set(groups.map((g) => g.id));
  const childGroups = new Map<string | null, Group[]>();
  for (const g of groups) {
    const parent = g.parentId !== null && known.has(g.parentId) ? g.parentId : null;
    childGroups.set(parent, [...(childGroups.get(parent) ?? []), g]);
  }
  const childHosts = new Map<string | null, HostView[]>();
  for (const h of hosts) {
    const parent = h.groupId !== null && known.has(h.groupId) ? h.groupId : null;
    childHosts.set(parent, [...(childHosts.get(parent) ?? []), h]);
  }

  const rows: TreeRow[] = [];
  const visited = new Set<string>();
  const walk = (parent: string | null, depth: number) => {
    for (const group of (childGroups.get(parent) ?? []).sort(byOrder)) {
      if (visited.has(group.id)) continue;
      visited.add(group.id);
      const expanded = !collapsed.has(group.id);
      const hostCount = childHosts.get(group.id)?.length ?? 0;
      rows.push({ kind: 'group', group, depth, expanded, hostCount });
      if (expanded) walk(group.id, depth + 1);
    }
    for (const host of (childHosts.get(parent) ?? []).sort(byLabel)) {
      rows.push({ kind: 'host', host, depth });
    }
  };
  walk(null, 0);
  return rows;
}

/** Caminho legível de cada grupo, ex.: "Clientes / ACME". */
export function groupPaths(groups: readonly Group[]): Map<string, string> {
  const byId = new Map(groups.map((g) => [g.id, g]));
  const paths = new Map<string, string>();
  const pathOf = (id: string, seen: Set<string>): string => {
    const cached = paths.get(id);
    if (cached !== undefined) return cached;
    const group = byId.get(id);
    if (!group) return '';
    seen.add(id);
    const parent =
      group.parentId !== null && !seen.has(group.parentId) ? pathOf(group.parentId, seen) : '';
    const path = parent ? `${parent} / ${group.name}` : group.name;
    paths.set(id, path);
    return path;
  };
  for (const g of groups) pathOf(g.id, new Set());
  return paths;
}

/** Grupos que não podem receber `groupId` (ele mesmo e descendentes). */
export function selfAndDescendants(groups: readonly Group[], groupId: string): Set<string> {
  const out = new Set([groupId]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const g of groups) {
      if (g.parentId !== null && out.has(g.parentId) && !out.has(g.id)) {
        out.add(g.id);
        grew = true;
      }
    }
  }
  return out;
}
