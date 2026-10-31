// Confirmação ao fechar com sessão aberta (T041; da 001 T101, FR-028).
import { expect, test } from '@playwright/test';
import { launchApp } from './helpers/launch';
import { exited, requestClose } from './helpers/system';
import { createVault, openLocalTab, waitPowerShellPrompt } from './helpers/ui';

test('fechar com sessão local aberta pede confirmação; cancelar mantém, confirmar fecha', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    const id = await openLocalTab(page, 'PowerShell');
    await waitPowerShellPrompt(page, id);

    await requestClose(page);
    const dialog = page.getByRole('dialog', { name: 'Fechar o OpenTerminal?' });
    await expect(dialog).toContainText('1 sessão aberta');
    await dialog.getByRole('button', { name: 'Cancelar' }).click();
    await expect(dialog).toBeHidden();
    expect(app.process.exitCode).toBeNull();

    await requestClose(page);
    await dialog.getByRole('button', { name: 'Fechar mesmo assim' }).click();
    await expect(exited(app.process)).resolves.toBe(0);
  } finally {
    await app.close();
  }
});

test('sem nada em andamento fecha direto', async () => {
  const app = await launchApp();
  try {
    await createVault(app.page);
    await requestClose(app.page);
    await expect(exited(app.process)).resolves.toBe(0);
  } finally {
    await app.close();
  }
});
