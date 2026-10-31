// Portão de qualidade da versão Wails: Go (vet + testes) e frontend (lint, tipos, testes).
// Para no primeiro passo que falhar.
import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '..');

const steps = [
  ['go', 'vet', './...'],
  ['go', 'test', './...'],
  ['node', '--test', 'scripts/check-tokens.test.mjs'],
  ...(existsSync(resolve(ROOT, 'scripts/check-tokens.mjs'))
    ? [['node', 'scripts/check-tokens.mjs']]
    : []),
  ['npm', '--prefix', 'frontend', 'run', 'lint'],
  ['npm', '--prefix', 'frontend', 'run', 'typecheck'],
  ['npm', '--prefix', 'frontend', 'test'],
  ['node', 'scripts/check-licenses.mjs'],
];

for (const [cmd, ...args] of steps) {
  const line = [cmd, ...args].join(' ');
  console.log(`\n> ${line}`);
  const r = spawnSync(cmd, args, {
    cwd: ROOT,
    stdio: 'inherit',
    shell: process.platform === 'win32',
  });
  if (r.status !== 0) {
    console.error(`\nFalhou: ${line}`);
    process.exit(r.status ?? 1);
  }
}
console.log('\nTudo verde.');
