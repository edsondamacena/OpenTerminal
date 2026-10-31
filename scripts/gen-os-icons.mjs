// Gera frontend/src/ui/osIcons.gen.ts com os logos de sistema do Simple Icons (CC0) (FR-178).
// Uso: node scripts/gen-os-icons.mjs <pasta node_modules com simple-icons@16.36.0>
// O app não depende do pacote: só os caminhos usados entram no repositório.
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const modules = process.argv[2];
if (!modules) {
  console.error('informe a pasta node_modules que contém simple-icons');
  process.exit(1);
}
const si = createRequire(join(modules, 'x.js'))('simple-icons');
const { version } = JSON.parse(
  readFileSync(join(modules, 'simple-icons', 'package.json'), 'utf8'),
);

// id do data-model → ícone do Simple Icons. Ausentes no pacote ficam no osIcons.tsx.
const MAP = {
  ubuntu: 'siUbuntu',
  debian: 'siDebian',
  centos: 'siCentos',
  rhel: 'siRedhat',
  fedora: 'siFedora',
  rocky: 'siRockylinux',
  almalinux: 'siAlmalinux',
  alpine: 'siAlpinelinux',
  arch: 'siArchlinux',
  opensuse: 'siOpensuse',
  linux: 'siLinux',
  freebsd: 'siFreebsd',
  openbsd: 'siOpenbsd',
  macos: 'siApple',
  mikrotik: 'siMikrotik',
  cisco: 'siCisco',
  huawei: 'siHuawei',
  juniper: 'siJunipernetworks',
  fortinet: 'siFortinet',
  openwrt: 'siOpenwrt',
};

const lines = [
  `// Gerado por scripts/gen-os-icons.mjs a partir de simple-icons@${version} (CC0-1.0). Não edite à mão.`,
  '// Marcas dos respectivos donos, usadas só para identificar o sistema do servidor.',
  '',
  'export const SIMPLE_OS_ICONS = {',
];
for (const [id, name] of Object.entries(MAP)) {
  const icon = si[name];
  if (!icon) throw new Error(`${name} não existe em simple-icons@${version}`);
  lines.push(`  ${id}: { title: ${JSON.stringify(icon.title)}, path: ${JSON.stringify(icon.path)} },`);
}
lines.push('} as const;', '');
const out = join(root, 'frontend', 'src', 'ui', 'osIcons.gen.ts');
writeFileSync(out, lines.join('\n'));
execFileSync('npx', ['prettier', '--write', out], { cwd: join(root, 'frontend'), shell: true });
console.log(`${String(Object.keys(MAP).length)} logos em ${out}`);
