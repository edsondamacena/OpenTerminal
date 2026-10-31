import type { ForwardRuleStatus } from '@shared/model/types';
import { t } from '../../i18n/pt-BR';

/** Mesma regra do principal: 127.0.0.0/8, ::1 ou localhost. */
export function isLoopback(address: string): boolean {
  return address === 'localhost' || address === '::1' || /^127\.\d+\.\d+\.\d+$/.test(address);
}

const endpoint = (host: string, port: number) =>
  host.includes(':') ? `[${host}]:${String(port)}` : `${host}:${String(port)}`;

export function describeRule(r: {
  bindAddress: string;
  bindPort: number;
  destHost: string;
  destPort: number;
}): string {
  return `${endpoint(r.bindAddress, r.bindPort)} → ${endpoint(r.destHost, r.destPort)}`;
}

export function statusText({ status, reason }: ForwardRuleStatus): string {
  const label = t.forwarding.status[status];
  const why = reason ? (t.forwarding.reasons[reason] ?? null) : null;
  return why ? `${label}: ${why}` : label;
}
