// US1 — cofre com senha mestra (T041; da 001 T040).
import { expect, test } from '@playwright/test';
import { launchApp } from './helpers/launch';
import { createVault, MASTER_PASSWORD, sidebarHosts, unlock, unlockForm } from './helpers/ui';

const NEW_PASSWORD = 'senha-teste-2';

test('criar, desbloquear, bloquear, trocar senha e apagar o cofre', async () => {
  // Primeira execução: criação com aviso de que não há recuperação (FR-006).
  let app = await launchApp();
  const { dataDir } = app;
  let { page } = app;
  await expect(page.getByText('Não há como recuperar a senha mestra')).toBeVisible();
  await createVault(page);
  await app.close();

  // Reiniciar → desbloqueio.
  app = await launchApp({ dataDir });
  ({ page } = app);
  try {
    await expect(unlockForm(page)).toBeVisible();

    // Senha errada → mensagem genérica, sem detalhes.
    await unlock(page, 'senha-errada');
    await expect(unlockForm(page).getByRole('alert')).toHaveText(
      'Senha incorreta ou cofre inválido.',
    );

    await unlock(page);
    await expect(sidebarHosts(page)).toBeVisible();

    // Bloqueio manual: o painel de desbloqueio ocupa a barra lateral.
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await expect(unlockForm(page)).toBeVisible();
    await expect(sidebarHosts(page)).toBeHidden();

    // Trocar a senha mestra.
    await unlock(page);
    await page.getByRole('button', { name: 'Configurações' }).click();
    const settings = page.getByRole('region', { name: 'Configurações' });
    await settings.getByRole('button', { name: 'Cofre e segurança' }).click();
    await settings.getByRole('button', { name: 'Trocar senha mestra…' }).click();
    const change = page.getByRole('dialog', { name: 'Trocar senha mestra' });
    await change.getByLabel('Senha atual', { exact: true }).fill(MASTER_PASSWORD);
    await change.getByLabel('Nova senha mestra', { exact: true }).fill(NEW_PASSWORD);
    await change.getByLabel('Confirme a nova senha', { exact: true }).fill(NEW_PASSWORD);
    await change.getByRole('button', { name: 'Salvar' }).click();
    await expect(page.getByText('Senha mestra alterada.')).toBeVisible({ timeout: 30_000 });
    await expect(change).toBeHidden();
    await page.keyboard.press('Escape');
    await expect(settings).toBeHidden();

    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await unlock(page);
    await expect(unlockForm(page).getByRole('alert')).toBeVisible();
    await unlock(page, NEW_PASSWORD);
    await expect(sidebarHosts(page)).toBeVisible();

    // Apagar o cofre exige confirmação dupla e volta à criação.
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await unlockForm(page).getByRole('button', { name: 'Esqueci a senha mestra' }).click();
    await page
      .getByRole('dialog', { name: 'Apagar o cofre' })
      .getByRole('button', { name: 'Continuar' })
      .click();
    const erase = page.getByRole('dialog', { name: 'Apagar o cofre' });
    const eraseButton = erase.getByRole('button', { name: 'Apagar cofre' });
    await expect(eraseButton).toBeDisabled();
    await erase.getByLabel('Para confirmar, digite APAGAR:').fill('APAGAR');
    await eraseButton.click();
    await expect(page.getByRole('button', { name: 'Criar cofre' })).toBeVisible();
  } finally {
    await app.close();
  }
});
