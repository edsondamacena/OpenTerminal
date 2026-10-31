// Popula um cofre de desenvolvimento usando os serviços Go da versão Wails.
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const result = spawnSync('go', ['run', './scripts/seed-hosts', ...process.argv.slice(2)], {
  cwd: root,
  stdio: 'inherit',
});

if (result.error) throw result.error;
process.exit(result.status ?? 1);
