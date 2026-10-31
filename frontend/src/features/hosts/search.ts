// Busca de hosts (FR-011): termos em rótulo, endereço e usuário, sem caixa nem acentos.
import type { HostView } from '@shared/model/types';

export interface SearchIndex {
  hosts: readonly HostView[];
  /** Texto normalizado por host, mesma ordem de `hosts`. */
  keys: readonly string[];
}

export function normalize(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLocaleLowerCase('pt-BR');
}

export function buildIndex(hosts: readonly HostView[]): SearchIndex {
  // Separador fora do alfabeto evita casar através de campos.
  return { hosts, keys: hosts.map((h) => normalize(`${h.label}\n${h.address}\n${h.username}`)) };
}

/** Termos da busca: espaço, hífen, ponto, barra e sublinhado separam ("pp 01", "olt-pp"). */
export function searchTerms(query: string): string[] {
  return normalize(query)
    .split(/[\s\-_./\\]+/)
    .filter((term) => term !== '');
}

/** Todos os termos precisam aparecer, em qualquer ordem: "pp 01" acha PROVEDOR-MT-PP-OLT-FKW-01. */
export function filterHosts(index: SearchIndex, query: string): HostView[] {
  const terms = searchTerms(query);
  if (terms.length === 0) return [...index.hosts];
  const out: HostView[] = [];
  index.keys.forEach((key, i) => {
    const host = index.hosts[i];
    if (host && terms.every((term) => key.includes(term))) out.push(host);
  });
  return out;
}
