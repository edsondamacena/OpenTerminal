import { describe, expect, it } from 'vitest';
import type { HostView } from '@shared/model/types';
import { buildIndex, filterHosts } from './search';

const host = (label: string, address = '10.0.0.1', username = 'admin') =>
  ({ id: label, label, address, username }) as HostView;

const hosts = [
  host('PROVEDOR-MT-PP-OLT-FKW-01'),
  host('PROVEDOR-MT-PP-OLT-FKW-02'),
  host('PROVEDOR-MT-PP-BNG-HW-01'),
  host('CLIENTE-CE-ARQ01-OLT01', '10.20.5.9'),
  host('Grafana'),
];
const labels = (q: string) => filterHosts(buildIndex(hosts), q).map((h) => h.label);

describe('filterHosts', () => {
  it('casa termos em qualquer ordem', () => {
    expect(labels('pp 01')).toEqual(['PROVEDOR-MT-PP-OLT-FKW-01', 'PROVEDOR-MT-PP-BNG-HW-01']);
    expect(labels('PROVEDOR-OLT-PP-01')).toEqual(['PROVEDOR-MT-PP-OLT-FKW-01']);
    expect(labels('olt pp')).toHaveLength(2);
  });

  it('aceita hífen, ponto e caixa variada', () => {
    expect(labels('pp-bng')).toEqual(['PROVEDOR-MT-PP-BNG-HW-01']);
    expect(labels('arq olt')).toEqual(['CLIENTE-CE-ARQ01-OLT01']);
    expect(labels('10.20')).toEqual(['CLIENTE-CE-ARQ01-OLT01']);
  });

  it('vazio devolve todos e termo ausente devolve nada', () => {
    expect(labels('  ')).toHaveLength(5);
    expect(labels('pp 03')).toEqual([]);
  });
});
