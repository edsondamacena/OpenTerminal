import { describe, expect, it, vi } from 'vitest';
import { createHighlighter, createLineJoiner } from './highlight';

const E = '\x1b';
// eslint-disable-next-line no-control-regex -- ESC é o que o realce precisa reconhecer
const strip = (s: string) => s.replace(/\x1b\[[0-9;]*m/g, '');

describe('realce de palavras de rede', () => {
  it('pinta estados, IPs, MACs e interfaces em texto puro', () => {
    const h = createHighlighter();
    const out = h(
      'GigabitEthernet0/0/1  up    10.0.0.1/30  aabb-ccdd-eeff\r\nEth-Trunk10 down\r\n',
    );
    expect(out).toContain(`${E}[34mGigabitEthernet0/0/1${E}[39m`);
    expect(out).toContain(`${E}[32mup${E}[39m`);
    expect(out).toContain(`${E}[36m10.0.0.1/30${E}[39m`);
    expect(out).toContain(`${E}[36maabb-ccdd-eeff${E}[39m`);
    expect(out).toContain(`${E}[31mdown${E}[39m`);
    expect(strip(out)).toBe(
      'GigabitEthernet0/0/1  up    10.0.0.1/30  aabb-ccdd-eeff\r\nEth-Trunk10 down\r\n',
    );
  });

  it('admin-down é aviso, não falha; palavras dentro de outras não casam', () => {
    const h = createHighlighter();
    const out = h('port 3 admin-down, setup update downstream');
    expect(out).toContain(`${E}[33madmin-down${E}[39m`);
    expect(h('GE0/0/2  *down  down')).toContain(`${E}[33m*down${E}[39m`);
    expect(out).not.toContain(`${E}[32mup`);
    expect(out).not.toContain(`${E}[31mdown${E}[39mstream`);
  });

  it('prompt do VRP em destaque no início da linha', () => {
    const h = createHighlighter();
    expect(h('\r\n<PROVEDOR-NE8K>')).toContain(`${E}[38;5;110m<PROVEDOR-NE8K>${E}[39m`);
    expect(h('\r\n[~R1-GigabitEthernet0/0/1]')).toContain(
      `${E}[38;5;110m[~R1-GigabitEthernet0/0/1]`,
    );
  });

  it('não mexe dentro de sequências de controle', () => {
    const h = createHighlighter();
    const out = h(`${E}[42D                                          ${E}[42D10.0.0.1 up`);
    expect(out.startsWith(`${E}[42D`)).toBe(true);
    expect(out).toContain(`${E}[42D${E}[36m10.0.0.1`);
  });

  it('servidor que já manda cor fica intacto daí em diante', () => {
    const h = createHighlighter();
    const colored = `${E}[01;32mnoc@vm${E}[00m:~$ ip a | grep up`;
    expect(h(colored)).toBe(colored);
    expect(h('eth0 up 10.0.0.1')).toBe('eth0 up 10.0.0.1');
  });

  it('tela cheia (vim, top) fica intacta até sair', () => {
    const h = createHighlighter();
    expect(h(`${E}[?1049hdown up`)).toBe(`${E}[?1049hdown up`);
    expect(h('down')).toBe('down');
    h(`${E}[?1049l`);
    expect(h('down')).toBe(`${E}[31mdown${E}[39m`);
  });
});

describe('OLT Furukawa (show onu info)', () => {
  const out = createHighlighter()(
    [
      'OLT-CENTRO-01[A]# show onu info',
      '   GPON1/1 |   1 |   Active | FRKW21988400 |   2400m  | - 29.9 dBm | PON17_ROUTER ',
      '   GPON1/1 |   2 |   Active | FRKW235f2ddd |   2896m  | - 23.0 dBm | PON17_630-10B ',
      '   GPON1/1 |   3 |   Active | FIOG5400852e |   2512m  | - 26.1 dBm | PREFEITURA_BRIDGE ',
      '   GPON1/1 |   4 | Inactive | HWTCccb44cb1 |   2209m  | -          | PON801_HUAWEI ',
      '   GPON1/1 |   5 |   Active | HWTC695be5b1 |   1474m  | - 24.0 dBm | PON801_HUAWEI ',
      '   GPON1/1 |   6 |   Active | HWTC83d9c8b3 |   2130m  | - 27.1 dBm | PON801_HUAWEI ',
      '   GPON1/1 |   7 |   Active | HWTCa37648b3 |   1356m  | - 27.0 dBm | PON801_HUAWEI ',
      '   GPON1/1 |   8 |   Active | FRKW2190d750 |   2117m  | - 24.5 dBm | PON1_423-41W ',
    ].join('\r\n'),
  );

  it('pinta interface, serial, perfil e prompt (31/10: cores do Edson, sem negrito)', () => {
    expect(out).toContain(`${E}[38;5;110mOLT-CENTRO-01[A]#${E}[39m`);
    expect(out).toContain(`${E}[38;5;109mGPON1/1${E}[39m`);
    expect(out).toContain(`${E}[38;5;117mFRKW21988400${E}[39m`);
    expect(out).toContain(`${E}[38;5;117mFIOG5400852e${E}[39m`);
    expect(out).toContain(`${E}[38;5;66mPON17_ROUTER${E}[39m`);
    expect(out).toContain(`${E}[38;5;66mPON1_423-41W${E}[39m`);
    expect(out).not.toContain(`${E}[1m`);
  });

  it('Active verde e Inactive cinza', () => {
    expect(out).toContain(`${E}[32mActive${E}[39m`);
    expect(out).toContain(`${E}[38;5;245mInactive${E}[39m`);
  });

  it('Rx Power: até −24 verde, −24,5 a −27 amarelo, abaixo de −27 vermelho', () => {
    expect(out).toContain(`${E}[32m- 23.0 dBm${E}[39m`);
    expect(out).toContain(`${E}[32m- 24.0 dBm${E}[39m`);
    expect(out).toContain(`${E}[33m- 24.5 dBm${E}[39m`);
    expect(out).toContain(`${E}[33m- 26.1 dBm${E}[39m`);
    expect(out).toContain(`${E}[33m- 27.0 dBm${E}[39m`);
    expect(out).toContain(`${E}[31m- 27.1 dBm${E}[39m`);
    expect(out).toContain(`${E}[31m- 29.9 dBm${E}[39m`);
  });

  it('Tx positivo fica sem cor e o texto não muda', () => {
    expect(createHighlighter()('Tx 2.5 dBm')).toBe('Tx 2.5 dBm');
    expect(strip(out)).toContain('|   2400m  | - 29.9 dBm | PON17_ROUTER ');
  });
});

describe('config de perfis da OLT Furukawa', () => {
  const cfg = [
    'traffic-profile PON3_ONU100 create',
    ' tcont 1 ',
    '  dba-profile 100MB',
    '   vlan-filter vid 103 untagged discard ',
    '   extended-vlan-tagging-operation PON3_ONU100 ',
    ' apply',
    '! traffic-profile, onu-profile e dba-profile',
    'extended-vlan-tagging-operation PON20_423-41w create',
    ' downstream-mode enable',
  ].join('\r\n');
  const out = createHighlighter()(cfg);

  it('palavra-chave sem negrito, nome do perfil na cor de perfil; apply, create e vid sem cor', () => {
    expect(out).toContain(`traffic-profile ${E}[38;5;66mPON3_ONU100${E}[39m create`);
    expect(out).toContain(`dba-profile ${E}[38;5;66m100MB${E}[39m`);
    expect(out).toContain(`extended-vlan-tagging-operation ${E}[38;5;66mPON20_423-41w${E}[39m`);
    expect(out).not.toContain(`${E}[1m`);
    expect(out).toContain('vid 103 untagged');
    expect(out).toContain('\r\n apply\r\n');
  });

  it('comentário inteiro apagado, sem realce dentro', () => {
    expect(out).toContain(`${E}[2m! traffic-profile, onu-profile e dba-profile${E}[22m`);
  });

  it('bridge e vlan da config ficam sem cor', () => {
    const h = createHighlighter();
    expect(h(' bridge 1 \r\n vlan 103')).toBe(' bridge 1 \r\n vlan 103');
  });

  it('downstream não vira "down" e o texto não muda', () => {
    expect(out).not.toContain(`${E}[31mdown`);
    expect(strip(out)).toBe(cfg);
  });
});

describe('interface gpon na config (31/10)', () => {
  it('interface gpon 2/7 com a cor das portas PON e o perfil sem negrito', () => {
    const out = createHighlighter()(
      'interface gpon 2/7\r\n olt service-profile model-name 423-41W/AC PON23_423-41W\r\n',
    );
    expect(out).toContain(`${E}[38;5;109minterface gpon 2/7${E}[39m`);
    expect(out).toContain(`${E}[38;5;66mPON23_423-41W${E}[39m`);
  });
});

describe('log de alarme da OLT (31/10)', () => {
  const out = createHighlighter()(
    [
      'CHAPADAO-DO-SUL-01[A](config-if[GPON2/15])# ',
      '2026/10/31 04:59:54  GPON[1502]: ONU(GPON2/15,3) DEACTIVATION (Reason: Admin control(ONU reset), SN:FRKW2190dd30)',
      '2026/10/31 05:01:07  GPON[1502]: ONU(GPON2/15,3) ACTIVATION (SN:FRKW2190dd30)',
    ].join('\r\n'),
  );

  it('DEACTIVATION vermelho e ACTIVATION verde', () => {
    expect(out).toContain(`${E}[31mDEACTIVATION${E}[39m`);
    expect(out).toContain(`${E}[32mACTIVATION${E}[39m`);
    expect(out).not.toContain(`DE${E}[32mACTIVATION`);
  });

  it('prompt em modo de configuração e porta PON no log', () => {
    expect(out).toContain(`${E}[38;5;110mCHAPADAO-DO-SUL-01[A](config-if[GPON2/15])#${E}[39m`);
    expect(out).toContain(`ONU(${E}[38;5;109mGPON2/15${E}[39m,3)`);
  });
});

describe('pedaços da mesma linha (31/10)', () => {
  it('"interface " e "gpon 2/7" em pedaços separados saem pintados juntos', () => {
    vi.useFakeTimers();
    const out: string[] = [];
    const h = createHighlighter();
    const j = createLineJoiner((d) => out.push(h(d)));
    j.push('interface ');
    j.push('gpon 2/7');
    j.push('\r\n no shutdown\r\n');
    expect(out.join('')).toContain(`${E}[38;5;109minterface gpon 2/7${E}[39m`);
    vi.useRealTimers();
  });

  it('linha completa sai na hora; o resto sai sozinho depois da espera (prompt, eco)', () => {
    vi.useFakeTimers();
    const out: string[] = [];
    const j = createLineJoiner((d) => out.push(d), 8);
    j.push('linha 1\r\nOLT#');
    expect(out).toEqual(['linha 1\r\n']);
    vi.advanceTimersByTime(8);
    expect(out).toEqual(['linha 1\r\n', 'OLT#']);
    j.push('a');
    j.flush();
    expect(out.join('')).toBe('linha 1\r\nOLT#a');
    vi.useRealTimers();
  });

  it('linha enorme sem fim não fica presa', () => {
    const out: string[] = [];
    const j = createLineJoiner((d) => out.push(d), 8, 10);
    j.push('x'.repeat(11));
    expect(out).toEqual(['x'.repeat(11)]);
  });
});
