// Logos de sistema no avatar do host (FR-178). Monocromáticos em `currentColor`, como o resto
// dos ícones; os caminhos vêm do Simple Icons (osIcons.gen.ts) e os ausentes lá ficam aqui.
import { OS_IDS, type Host, type OsId } from '@shared/model/types';
import { NetworkIcon } from './icons';
import { SIMPLE_OS_ICONS } from './osIcons.gen';

/** Quatro janelas, sem a perspectiva do logo atual: o Simple Icons não traz o do Windows. */
const WINDOWS_PATH = 'M3 3h8.5v8.5H3zM12.5 3H21v8.5h-8.5zM3 12.5h8.5V21H3zM12.5 12.5H21V21h-8.5z';

const LABELS: Record<OsId, string> = {
  ubuntu: 'Ubuntu',
  debian: 'Debian',
  centos: 'CentOS',
  rhel: 'Red Hat Enterprise Linux',
  fedora: 'Fedora',
  rocky: 'Rocky Linux',
  almalinux: 'AlmaLinux',
  alpine: 'Alpine Linux',
  arch: 'Arch Linux',
  opensuse: 'openSUSE',
  oracle: 'Oracle Linux',
  amazon: 'Amazon Linux',
  linux: 'Linux',
  freebsd: 'FreeBSD',
  openbsd: 'OpenBSD',
  windows: 'Windows',
  macos: 'macOS',
  mikrotik: 'MikroTik',
  cisco: 'Cisco',
  huawei: 'Huawei',
  juniper: 'Juniper',
  hpe: 'HPE / Aruba (Comware)',
  fortinet: 'Fortinet',
  openwrt: 'OpenWrt',
};

/** O Go grava `osIcon` a partir do T126; até lá o seletor fica escondido para não perder a escolha. */
export const OS_ICON_EDITABLE = false as boolean;

export const osLabel = (id: OsId): string => LABELS[id];

export const OS_OPTIONS = OS_IDS.map((id) => ({ id, label: LABELS[id] })).sort((a, b) =>
  a.label.localeCompare(b.label, 'pt-BR'),
);

function pathOf(id: OsId): string | null {
  if (id in SIMPLE_OS_ICONS) return SIMPLE_OS_ICONS[id as keyof typeof SIMPLE_OS_ICONS].path;
  // Oracle e Amazon Linux não estão no Simple Icons: usam o pinguim.
  if (id === 'oracle' || id === 'amazon') return SIMPLE_OS_ICONS.linux.path;
  if (id === 'windows') return WINDOWS_PATH;
  return null;
}

/** Ícone efetivo do host: a escolha do usuário ou, em `auto`, o detectado. */
export function hostOs(host: Pick<Host, 'osIcon' | 'detectedOs'>): OsId | null {
  const chosen = host.osIcon ?? 'auto';
  return chosen === 'auto' ? (host.detectedOs ?? null) : chosen;
}

export function OsLogo({ id, size = 16 }: { id: OsId; size?: number }) {
  const path = pathOf(id);
  // Sem logo próprio (HPE/Comware): o ícone genérico de equipamento de rede.
  if (!path) return <NetworkIcon width={size} height={size} aria-hidden />;
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d={path} />
    </svg>
  );
}
