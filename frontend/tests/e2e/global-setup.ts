// Compila o build de testes uma vez por execução (tag e2e: CDP + TestService).
import { execFileSync } from 'node:child_process';
import { REPO_ROOT } from './helpers/paths';

export default function globalSetup(): void {
  if (process.env.OPENTERMINAL_E2E_SKIP_BUILD === '1') return;
  execFileSync('wails', ['build', '-tags', 'e2e', '-o', 'openterminal-e2e.exe'], {
    cwd: REPO_ROOT,
    stdio: 'inherit',
  });
}
