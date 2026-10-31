// US6 — navegador SFTP, envio, download, pastas e conflito (portado da 001, T097).
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { SSHD } from '../../../scripts/sshd.mjs';
import { launchApp, testService } from './helpers/launch';
import { acceptHostKey, addHost, createVault, hostItem } from './helpers/ui';

/** Troca o caminho que os diálogos nativos devolvem em E2E. */
async function dialogReturns(page: Page, path: string) {
  await testService(page, 'SetDialogPath', path);
}

const row = (page: Page, name: string) =>
  page.getByRole('row').filter({ has: page.getByText(name, { exact: true }) });

test('navegar, enviar, baixar, criar e excluir pasta, conflito', async () => {
  const app = await launchApp();
  const { page } = app;
  const local = mkdtempSync(join(tmpdir(), 'openterminal-e2e-sftp-'));
  const upload = join(local, 'enviar.txt');
  writeFileSync(upload, 'conteúdo de teste');
  const unique = `pasta e2e ${Date.now().toString(36)}`;
  try {
    await createVault(page);
    await addHost(page, {
      label: 'pw-host',
      address: SSHD.host,
      port: SSHD.port,
      username: 'pwuser',
      password: 'pw-test-123',
    });
    await hostItem(page, 'pw-host').click({ button: 'right' });
    await page.getByRole('menuitem', { name: 'Abrir SFTP' }).click();
    await acceptHostKey(page);

    const view = page.getByRole('tabpanel', { name: 'pw-host — SFTP' });
    await expect(view).toBeVisible();
    const crumbs = view.getByRole('navigation', { name: 'Local remoto' });
    await expect(crumbs.locator('[aria-current="location"]')).toHaveText('pwuser');

    // Enviar arquivo (diálogo de seleção simulado).
    await dialogReturns(page, upload);
    await view.getByRole('button', { name: 'Enviar arquivos…' }).click();
    await expect(row(page, 'enviar.txt')).toBeVisible();

    // Criar pasta, entrar e sair.
    await view.getByRole('button', { name: 'Nova pasta' }).click();
    const mkdir = page.getByRole('dialog', { name: 'Nova pasta' });
    await mkdir.getByLabel('Nome da pasta').fill(unique);
    await mkdir.getByRole('button', { name: 'Salvar' }).click();
    await row(page, unique).dblclick();
    await expect(crumbs.locator('[aria-current="location"]')).toHaveText(unique);
    await expect(view.getByText('Pasta vazia.')).toBeVisible();
    await view.getByRole('button', { name: 'Pasta acima' }).click();
    await expect(crumbs.locator('[aria-current="location"]')).toHaveText('pwuser');

    // Baixar para a pasta escolhida no diálogo simulado.
    const dest = mkdtempSync(join(tmpdir(), 'openterminal-e2e-dl-'));
    await dialogReturns(page, dest);
    await row(page, 'enviar.txt').click();
    await view.getByRole('button', { name: 'Baixar…' }).click();
    const transfers = view.getByRole('region', { name: 'Transferências' });
    await expect(transfers.locator('li[data-state="completed"]')).toHaveCount(2);
    expect(readFileSync(join(dest, 'enviar.txt'), 'utf8')).toBe('conteúdo de teste');

    // Conflito ao enviar de novo: pular.
    await dialogReturns(page, upload);
    await view.getByRole('button', { name: 'Enviar arquivos…' }).click();
    const conflict = page.getByRole('dialog', { name: 'O arquivo já existe' });
    await expect(conflict).toContainText('"enviar.txt" já existe');
    await conflict.getByRole('button', { name: 'Pular' }).click();
    await expect(transfers.locator('li[data-state="skipped"]')).toHaveCount(1);

    // Excluir com confirmação.
    for (const name of [unique, 'enviar.txt']) {
      await row(page, name).click({ button: 'right' });
      await page.getByRole('menuitem', { name: 'Excluir' }).click();
      const confirm = page.getByRole('dialog', { name: 'Excluir itens' });
      await expect(confirm).toContainText(name);
      await confirm.getByRole('button', { name: 'Excluir' }).click();
      await expect(row(page, name)).toHaveCount(0);
    }
  } finally {
    await app.close();
  }
});
