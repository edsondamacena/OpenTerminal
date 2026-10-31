// Verifica as licenças das dependências de produção (Princípio I).
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

// Unlicense: domínio público, aprovada pela OSI (usada por tweetnacl, dependência do ssh2).
const ALLOWED = new Set([
  'MIT',
  'ISC',
  'Apache-2.0',
  'BSD-2-Clause',
  'BSD-3-Clause',
  '0BSD',
  'Unlicense',
]);
const root = process.cwd();
const frontend = join(root, 'frontend');

function readPkg(dir) {
  return JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8'));
}

function licenseOf(pkg) {
  if (typeof pkg.license === 'string') return pkg.license;
  if (pkg.license && typeof pkg.license.type === 'string') return pkg.license.type;
  if (Array.isArray(pkg.licenses)) return pkg.licenses.map((l) => l.type).join(' OR ');
  return 'UNKNOWN';
}

function isAllowed(expr) {
  // Aceita expressões SPDX simples com OR se alguma alternativa for permitida.
  return expr
    .replace(/[()]/g, '')
    .split(/\s+OR\s+/)
    .some((alt) => alt.split(/\s+AND\s+/).every((l) => ALLOWED.has(l.trim())));
}

const seen = new Map();
function walk(name, fromDir) {
  let dir = fromDir;
  for (;;) {
    const candidate = join(dir, 'node_modules', name);
    try {
      const pkg = readPkg(candidate);
      const key = `${pkg.name}@${pkg.version}`;
      if (seen.has(key)) return;
      seen.set(key, licenseOf(pkg));
      for (const dep of Object.keys({ ...pkg.dependencies, ...pkg.optionalDependencies })) {
        walk(dep, candidate);
      }
      return;
    } catch {
      const parent = join(dir, '..');
      if (parent === dir || !dir.startsWith(root)) return;
      dir = parent;
    }
  }
}

for (const dep of Object.keys(readPkg(root).dependencies ?? {})) walk(dep, root);
const frontendDirect = [];
for (const dep of Object.keys(readPkg(frontend).dependencies ?? {})) {
  const installed = readPkg(join(frontend, 'node_modules', dep));
  frontendDirect.push(`${installed.name}@${installed.version}`);
  walk(dep, frontend);
}

const rows = [...seen].sort(([a], [b]) => a.localeCompare(b));
const bad = rows.filter(([, lic]) => !isAllowed(lic));
for (const [name, lic] of rows) console.log(`${isAllowed(lic) ? 'ok ' : 'BAD'} ${name} ${lic}`);
const documentation = readFileSync(join(root, 'docs', 'licenses.md'), 'utf8');
const undocumented = frontendDirect.filter((entry) => {
  const separator = entry.lastIndexOf('@');
  const name = entry.slice(0, separator);
  const version = entry.slice(separator + 1);
  return !documentation.includes(`| \`${name}\` ${version} |`);
});
const requiredContent = [
  'JetBrains Mono',
  'Cascadia Code',
  'OpenTerminal Dark',
  'Dracula',
  'Nord',
  'Solarized Dark',
  'Solarized Light',
  'One Dark',
  'Tokyo Night',
  'Catppuccin Mocha',
];
const missingContent = requiredContent.filter((item) => !documentation.includes(`| ${item}`));
for (const entry of undocumented) console.error(`DOC ausente: ${entry}`);
for (const item of missingContent) console.error(`CONTEÚDO ausente: ${item}`);
if (bad.length > 0 || undocumented.length > 0 || missingContent.length > 0) {
  console.error(
    `\nFalhas: ${bad.length} licença(s), ${undocumented.length} dependência(s) npm sem documentação, ${missingContent.length} conteúdo(s) sem origem.`,
  );
  process.exit(1);
}
console.log(
  `\n${rows.length} dependências de produção verificadas; ${frontendDirect.length} dependências diretas do frontend documentadas.`,
);
