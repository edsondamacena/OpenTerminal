// Uma janela e um processo (T041, FR-111; regressão do bug 1: shell local reabria o app).
import { spawn } from 'node:child_process';
import { expect, test } from '@playwright/test';
import { launchApp, testService } from './helpers/launch';
import { E2E_EXE } from './helpers/paths';
import { exited, isMinimised, processesOf } from './helpers/system';
import { createVault, openLocalTab, waitPowerShellPrompt } from './helpers/ui';

test('segunda execução não abre janela e traz a existente para frente', async () => {
  const app = await launchApp();
  const { page } = app;
  const pid = app.process.pid ?? 0;
  try {
    await createVault(page);
    await page.evaluate(() => {
      const go = (
        window as unknown as { go: { bridge: Record<string, Record<string, () => Promise<void>>> } }
      ).go;
      return go.bridge.AppService?.WindowMinimise?.();
    });
    await expect.poll(() => isMinimised(pid)).toBe(true);

    // Mesmo diretório de dados = mesma trava de instância única.
    const second = spawn(E2E_EXE, [], {
      env: {
        ...process.env,
        OPENTERMINAL_DATA_DIR: app.dataDir,
        OPENTERMINAL_E2E_CDP_PORT: String(9000 + Math.floor(Math.random() * 300)),
      },
      stdio: 'ignore',
    });
    await expect(exited(second)).resolves.toBe(0);
    await expect.poll(() => isMinimised(pid)).toBe(false);
    expect(processesOf(E2E_EXE)).toEqual([app.process.pid]);
  } finally {
    await app.close();
  }
});

test('abrir shells e sessões não cria outro processo do app', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    const ps = await openLocalTab(page, 'PowerShell');
    await waitPowerShellPrompt(page, ps);
    await openLocalTab(page, 'Prompt de Comando');
    await expect.poll(() => testService<number>(page, 'SessionCount')).toBe(2);
    expect(processesOf(E2E_EXE)).toEqual([app.process.pid]);
    expect(
      app.browser
        .contexts()
        .flatMap((c) => c.pages())
        .filter((p) => !p.url().startsWith('devtools:')),
    ).toHaveLength(1);
  } finally {
    await app.close();
  }
});
