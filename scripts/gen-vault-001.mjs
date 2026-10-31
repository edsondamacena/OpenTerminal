// Gera a fixture do cofre 001 usando o implemento Go compatível com o formato legado.
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const result = spawnSync('go', ['run', './scripts/gen-vault-001', ...process.argv.slice(2)], {
  cwd: root,
  stdio: 'inherit',
});

if (result.error) throw result.error;
process.exit(result.status ?? 1);
