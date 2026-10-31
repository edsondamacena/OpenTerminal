// US4 — terminal local com o cofre bloqueado (T041; da 001 T080).
import { expect, test } from '@playwright/test';
import { launchApp } from './helpers/launch';
import {
  createVault,
  expectTerminalLine,
  openLocalTab,
  tab,
  terminalPanel,
  typeInTerminal,
  unlockForm,
  waitPowerShellPrompt,
} from './helpers/ui';

test('PowerShell abre com o cofre bloqueado e mostra o encerramento', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await expect(unlockForm(page)).toBeVisible();

    const id = await openLocalTab(page, 'PowerShell');
    await waitPowerShellPrompt(page, id);
    await typeInTerminal(page, 'PowerShell', 'echo ok\r');
    await expectTerminalLine(page, id, 'ok');

    await typeInTerminal(page, 'PowerShell', 'exit\r');
    const panel = terminalPanel(page, 'PowerShell');
    await expect(panel.getByRole('status')).toContainText('Processo encerrado', {
      timeout: 20_000,
    });
    await expect(tab(page, 'PowerShell')).toHaveAttribute('title', /Processo encerrado/);

    // Ctrl+Shift+T abre outro PowerShell (shell padrão); Prompt de Comando pelo menu.
    await page.keyboard.press('Control+Shift+T');
    await expect(page.getByRole('tab', { name: 'PowerShell' })).toHaveCount(2);
    const cmd = await openLocalTab(page, 'Prompt de Comando');
    await typeInTerminal(page, 'Prompt de Comando', 'echo cmd-ok\r');
    await expectTerminalLine(page, cmd, 'cmd-ok');
  } finally {
    await app.close();
  }
});
