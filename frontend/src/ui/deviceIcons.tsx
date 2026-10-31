// Ícones de equipamento de rede no avatar do host. Sem escolha, o padrão é o do protocolo
// (SSH ou telnet).
import { DEVICE_ICONS, type DeviceIconId, type Host } from '@shared/model/types';
import {
  BngIcon,
  EdgeIcon,
  FirewallIcon,
  OltIcon,
  OnuIcon,
  RadioIcon,
  RouterIcon,
  ServerIcon,
  SshIcon,
  SwitchIcon,
  TelnetIcon,
  WifiIcon,
} from './icons';

export { DEVICE_ICONS, type DeviceIconId };

const GLYPHS: Record<DeviceIconId, typeof SshIcon> = {
  router: RouterIcon,
  bng: BngIcon,
  switch: SwitchIcon,
  olt: OltIcon,
  onu: OnuIcon,
  server: ServerIcon,
  firewall: FirewallIcon,
  radio: RadioIcon,
  wifi: WifiIcon,
  edge: EdgeIcon,
  ssh: SshIcon,
  telnet: TelnetIcon,
};

/** Ícone efetivo: o escolhido ou, sem escolha, o do protocolo. */
export function hostDeviceIcon(host: Pick<Host, 'deviceIcon' | 'protocol'>): DeviceIconId {
  if (host.deviceIcon) return host.deviceIcon;
  return host.protocol === 'telnet' ? 'telnet' : 'ssh';
}

export function DeviceLogo({
  id,
  size = 16,
  className,
}: {
  id: DeviceIconId;
  size?: number;
  className?: string | undefined;
}) {
  const Glyph = GLYPHS[id];
  return <Glyph width={size} height={size} className={className} />;
}
