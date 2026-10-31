// Garante que o binário de release não tem nada do build de testes (research R18):
// nem a porta de depuração do WebView2, nem o TestService.
// Uso: node scripts/check-release.mjs [--no-build]
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '..');
const EXE = resolve(ROOT, 'build', 'bin', 'openterminal.exe');
const FORBIDDEN = ['remote-debugging-port', 'TestService', 'openterminal/internal/bridge/testsvc'];

if (!process.argv.includes('--no-build')) {
  execFileSync('wails', ['build', '-trimpath', '-ldflags', '-s -w'], {
    cwd: ROOT,
    stdio: 'inherit',
  });
}

const bin = readFileSync(EXE);
const found = FORBIDDEN.filter((s) => bin.includes(Buffer.from(s, 'utf8')));
if (found.length > 0) {
  console.error(`Binário de release contém código de teste: ${found.join(', ')}`);
  process.exit(1);
}
console.log(`Release ok: ${EXE} (${(bin.length / 2 ** 20).toFixed(1)} MiB) sem código de teste.`);
