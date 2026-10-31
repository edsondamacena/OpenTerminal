import { resolve } from 'node:path';

export const REPO_ROOT = resolve(import.meta.dirname, '../../../..');
/** Binário do build de testes, gerado pelo global-setup. */
export const E2E_EXE = resolve(REPO_ROOT, 'build/bin/openterminal-e2e.exe');
