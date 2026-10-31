// Realce de palavras de rede (estilo MobaXterm/SecureCRT) para saídas que chegam sem cor:
// Huawei VRP, Mikrotik, OLTs. Só pinta texto puro, nunca dentro de sequências de controle, e
// sai do caminho quando o servidor já usa cores próprias ou abre uma tela cheia (vim, top).

const ESC = '\x1b';
const FG_RESET = `${ESC}[39m`;
// Paleta da OLT (31/10, pedido do Edson): cores de 256 tons, que aparecem no fundo escuro (o azul
// ANSI 34 do tema sumia e o "gpon 2/7" parecia sem cor). Sem negrito em prompt e perfis.
const PROMPT = `${ESC}[38;5;110m`; // nome do host no prompt: azul suave
const PON_IF = `${ESC}[38;5;109m`; // GPON1/1, interface gpon 2/7: verde-azulado suave
const PROFILE = `${ESC}[38;5;66m`; // perfis (PON801_HUAWEI, nome do dba-profile): o mesmo, mais escuro
const SERIAL = `${ESC}[38;5;117m`; // serial da ONU: azul claro
const INACTIVE = `${ESC}[38;5;245m`; // ONU Inactive: cinza

interface Rule {
  re: string;
  /** Cor fixa, ou função que escolhe pela palavra (null deixa sem cor). */
  sgr: string | ((match: string) => string | null);
  reset?: string;
}

/** Potência óptica recebida (31/10, faixas do Edson): até −24 dBm verde (ok), de −24,5 a −27
 *  amarelo (atenção), abaixo de −27 vermelho (GPON classe B+ tem sensibilidade em −28 dBm). */
export function rxPowerColor(match: string): string | null {
  const value = Number(match.replace(/\s+/g, '').replace(/dBm$/i, ''));
  if (!Number.isFinite(value)) return null;
  if (value > -24.5) return `${ESC}[32m`;
  if (value >= -27) return `${ESC}[33m`;
  return `${ESC}[31m`;
}

// Ordem importa: a primeira regra que casar vence (admin-down antes de down, por exemplo).
const RULES: Rule[] = [
  // Prompt do VRP/Comware: <HUAWEI>, [~HUAWEI-GigabitEthernet0/0/1], [*R1].
  {
    re: String.raw`(?<=^|[\r\n])(?:<[\w.\-]+>|\[[~*]?[\w.\-]+(?:-[\w./:\-]+)?\]|[\w.\-]+(?:\[[\w\-]+\])?(?:\([\w\-\[\]\/.]+\))?[#>](?= |$))`,
    sgr: PROMPT,
  },
  // Config da OLT (Furukawa): comentário "!" apagado; perfis principais e o nome de cada um.
  { re: String.raw`(?<=^|[\r\n])[ \t]*!.*`, sgr: `${ESC}[2m`, reset: `${ESC}[22m` },
  // Nome do perfil depois de dba-profile/traffic-profile...: cor de perfil, sem negrito (a palavra-chave fica normal).
  {
    re: String.raw`(?<=\b(?:traffic-profile|extended-vlan-tagging-operation|onu-profile|dba-profile) )[\w.\-]+`,
    sgr: PROFILE,
  },
  // Log da OLT: ONU caiu em vermelho, voltou em verde (DEACTIVATION antes de ACTIVATION).
  { re: String.raw`\bDEACTIVATION\b`, sgr: `${ESC}[31m` },
  { re: String.raw`\bACTIVATION\b`, sgr: `${ESC}[32m` },
  // ONU desligada: cinza (antes de "active", que é verde).
  { re: String.raw`\b(?:Inactive|inactive|INACTIVE)\b`, sgr: INACTIVE },
  { re: String.raw`\b(?:admin(?:istratively)?[ \-]?down|ADM DOWN)\b|\*down\b`, sgr: `${ESC}[33m` },
  {
    re: String.raw`\b(?:down|DOWN|Down|errors?|ERRORS?|Error|fail(?:ed|ure)?|FAIL(?:ED)?|Failed|timeout|Timeout|timed out|LOS|LOFI?|dying[ \-]gasp|Dying[ \-]gasp|unreachable|Unreachable|denied|refused|offline|Offline|OFFLINE|critical|CRITICAL)\b`,
    sgr: `${ESC}[31m`,
  },
  {
    re: String.raw`\b(?:warn(?:ing)?|WARN(?:ING)?|Warning|degraded|flapping|deactivated|shutdown|blocked|BLOCKED)\b`,
    sgr: `${ESC}[33m`,
  },
  {
    re: String.raw`\b(?:up|UP|Up|online|Online|ONLINE|established|Established|ESTABLISHED|connected|active|Active|ACTIVE|working|success(?:ful)?|forwarding|FORWARDING|ok|OK)\b`,
    sgr: `${ESC}[32m`,
  },
  // Potência óptica: "-23.0 dBm", "- 29.9 dBm" (Furukawa põe espaço depois do sinal).
  { re: String.raw`-\s?\d{1,2}(?:\.\d{1,2})?\s?dBm\b`, sgr: rxPowerColor },
  // Serial de ONU: fabricante (4 letras) + 8 hexadecimais. FRKW…, HWTC…, FIOG…, ZTEG…
  { re: String.raw`\b[A-Z]{4}[0-9A-Fa-f]{8}\b`, sgr: SERIAL },
  // Perfil de ONU no padrão da casa: PON17_ROUTER, PON117_HUAWEI.
  { re: String.raw`\bPON\d+_[\w\-]+`, sgr: PROFILE },
  // MAC: aa:bb:cc:dd:ee:ff, aa-bb-..., aabb-ccdd-eeff (Huawei), aabb.ccdd.eeff (Cisco).
  {
    re: String.raw`\b(?:[0-9A-Fa-f]{2}[:\-]){5}[0-9A-Fa-f]{2}\b|\b[0-9A-Fa-f]{4}(?<sep>[.\-])[0-9A-Fa-f]{4}\k<sep>[0-9A-Fa-f]{4}\b`,
    sgr: `${ESC}[36m`,
  },
  // IPv4 com máscara opcional.
  { re: String.raw`\b(?:\d{1,3}\.){3}\d{1,3}(?:\/\d{1,2})?\b`, sgr: `${ESC}[36m` },
  // Portas PON: GPON1/1, "interface gpon 2/7" inteiro (o interface junto), XGS-PON0/1 (verde-azulado suave, mais visível que o azul do tema).
  {
    re: String.raw`\b(?:interface )?(?:XGS-?PON|XG-?PON|GPON|EPON|gpon|epon|xpon|pon)[ ]?\d+(?:[\/:.]\d+)*\b`,
    sgr: PON_IF,
  },
  // Outras interfaces: GigabitEthernet0/0/1, XGE0/1/2, Eth-Trunk10, Vlanif100, ether1, sfp-sfpplus1.
  {
    re: String.raw`\b(?:(?:X|Ten-|100|25|40)?GigabitEthernet|XGE|GE|Eth-Trunk|Vlanif|Vlan-interface|LoopBack|Ethernet|Bridge-Aggregation|Port-channel|port-channel|ether|sfp-sfpplus|sfp|combo|wlan|bond|pppoe-out)[ ]?\d+(?:[\/:.]\d+)*\b`,
    sgr: `${ESC}[34m`,
  },
];

// Cada regra vira um grupo nomeado r0, r1…; o grupo que casou diz a cor.
const COMBINED = new RegExp(RULES.map((r, i) => `(?<r${String(i)}>${r.re})`).join('|'), 'g');

// Sequências de controle completas (CSI, OSC, ESC + 1 caractere).
// eslint-disable-next-line no-control-regex -- ESC é o que o realce precisa reconhecer
const CONTROL = /\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]/g;
// eslint-disable-next-line no-control-regex -- ESC é o que o realce precisa reconhecer
const SGR = /\x1b\[([0-9;]*)m/g;

/** O trecho define cor de texto ou fundo (30–38, 40–48, 90–97, 100–107)? */
function setsColor(data: string): boolean {
  for (const m of data.matchAll(SGR)) {
    for (const p of (m[1] ?? '').split(';')) {
      const n = Number(p);
      if (
        (n >= 30 && n <= 38) ||
        (n >= 40 && n <= 48) ||
        (n >= 90 && n <= 97) ||
        (n >= 100 && n <= 107)
      ) {
        return true;
      }
    }
  }
  return false;
}

function paint(text: string): string {
  return text.replace(COMBINED, (...args: unknown[]) => {
    const match = args[0] as string;
    const groups = args[args.length - 1] as Record<string, string | undefined>;
    const rule = RULES.find((_, i) => groups[`r${String(i)}`] !== undefined);
    if (!rule) return match;
    const sgr = typeof rule.sgr === 'string' ? rule.sgr : rule.sgr(match);
    return sgr ? `${sgr}${match}${rule.reset ?? FG_RESET}` : match;
  });
}

/** Cria o realce de uma sessão: guarda se o servidor já pinta e se está em tela cheia. */
export function createHighlighter(): (data: string) => string {
  let serverColors = false;
  let altScreen = false;
  return (data) => {
    if (data.includes(ESC)) {
      if (data.includes(`${ESC}[?1049h`) || data.includes(`${ESC}[?47h`)) altScreen = true;
      if (data.includes(`${ESC}[?1049l`) || data.includes(`${ESC}[?47l`)) altScreen = false;
      if (setsColor(data)) serverColors = true;
    }
    if (serverColors || altScreen) return data;
    if (!data.includes(ESC)) return paint(data);
    // Pinta só o texto entre as sequências; um ESC incompleto no fim fica como veio.
    let out = '';
    let last = 0;
    for (const m of data.matchAll(CONTROL)) {
      out += paint(data.slice(last, m.index)) + m[0];
      last = m.index + m[0].length;
    }
    const tail = data.slice(last);
    out += tail.includes(ESC) ? tail : paint(tail);
    return out;
  };
}

/** Junta os pedaços da linha antes de pintar (31/10): a OLT manda "interface " num pedaço e
 *  "gpon 2/7" no seguinte, e o realce de cada pedaço sozinho perdia o "interface". Linhas completas
 *  saem na hora; o resto da última linha espera até `waitMs` pelo próximo pedaço (eco de digitação
 *  e prompt atrasam no máximo isso). Linha sem fim maior que `maxHold` sai sem esperar. */
export function createLineJoiner(
  emit: (data: string) => void,
  waitMs = 8,
  maxHold = 4096,
): { push: (data: string) => void; flush: () => void } {
  let pending = '';
  let timer: ReturnType<typeof setTimeout> | undefined;
  const flush = () => {
    if (timer !== undefined) clearTimeout(timer);
    timer = undefined;
    if (pending) {
      const out = pending;
      pending = '';
      emit(out);
    }
  };
  const push = (data: string) => {
    if (timer !== undefined) clearTimeout(timer);
    timer = undefined;
    const all = pending + data;
    const cut = Math.max(all.lastIndexOf('\n'), all.lastIndexOf('\r')) + 1;
    pending = all.slice(cut);
    if (cut > 0) emit(all.slice(0, cut));
    if (pending.length > maxHold) flush();
    else if (pending) timer = setTimeout(flush, waitMs);
  };
  return { push, flush };
}
