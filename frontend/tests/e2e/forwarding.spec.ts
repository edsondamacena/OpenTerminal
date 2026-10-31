// US6 — encaminhamento de portas local (portado da 001, T097).
import { createConnection } from 'node:net';
import { expect, test, type Page } from '@playwright/test';
import { SSHD } from '../../../scripts/sshd.mjs';
import { launchApp } from './helpers/launch';
import { acceptHostKey, addHost, createVault, hostItem, unlockForm } from './helpers/ui';

function echo(port: number, text: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const socket = createConnection({ host: '127.0.0.1', port }, () => socket.write(text));
    let got = '';
    socket.on('data', (d: Buffer) => {
      got += d.toString('utf8');
      if (got.length >= text.length) {
        socket.end();
        resolve(got);
      }
    });
    socket.on('error', reject);
  });
}

async function addRule(
  page: Page,
  rule: { bindAddress?: string; bindPort: number; destHost: string; destPort: number },
) {
  const panel = page.getByRole('dialog', { name: 'Encaminhamento de portas — pw-host' });
  await panel.getByRole('button', { name: 'Nova regra' }).click();
  const form = panel.getByRole('form', { name: 'Nova regra' });
  if (rule.bindAddress) await form.getByLabel('Endereço local').fill(rule.bindAddress);
  await form.getByLabel('Porta local').fill(String(rule.bindPort));
  await form.getByLabel('Host de destino').fill(rule.destHost);
  await form.getByLabel('Porta de destino').fill(String(rule.destPort));
  await form.getByRole('button', { name: 'Salvar' }).click();
}

test('regra de encaminhamento: ativar, desativar, expor na rede e cofre bloqueado', async () => {
  const app = await launchApp();
  const { page } = app;
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
    await page.getByRole('menuitem', { name: 'Encaminhamento de portas' }).click();
    const panel = page.getByRole('dialog', { name: 'Encaminhamento de portas — pw-host' });

    await addRule(page, { bindPort: 17010, destHost: 'echo', destPort: 7000 });
    const rule = panel.getByRole('listitem', { name: '127.0.0.1:17010 → echo:7000' });
    await expect(rule.getByRole('status')).toHaveText('Inativa');

    // Ativar (primeira conexão pede a host key) e trocar dados pela porta local.
    await rule.getByRole('button', { name: 'Ativar' }).click();
    await acceptHostKey(page);
    await expect(rule.getByRole('status')).toHaveText('Ativa');
    expect(await echo(17010, 'eco pelo túnel')).toBe('eco pelo túnel');

    await rule.getByRole('button', { name: 'Desativar' }).click();
    await expect(rule.getByRole('status')).toHaveText('Inativa');

    // Endereço fora do loopback exige confirmação de aviso (FR-025).
    await addRule(page, {
      bindAddress: '0.0.0.0',
      bindPort: 17011,
      destHost: 'echo',
      destPort: 7000,
    });
    const warning = page.getByRole('dialog', { name: 'Expor a porta na rede?' });
    await expect(warning).toContainText('outros computadores da rede');
    await warning.getByRole('button', { name: 'Expor na rede' }).click();
    await expect(panel.getByRole('listitem', { name: '0.0.0.0:17011 → echo:7000' })).toBeVisible();

    // Com o cofre bloqueado, o encaminhamento ativo segue visível e desativável (FR-003).
    await rule.getByRole('button', { name: 'Ativar' }).click();
    await expect(rule.getByRole('status')).toHaveText('Ativa');
    await panel.getByRole('button', { name: 'Fechar' }).last().click();
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await expect(unlockForm(page)).toBeVisible();

    // O indicador fica na barra de status (FR-172).
    await page
      .getByRole('contentinfo', { name: 'Barra de status' })
      .getByRole('button', { name: '1 encaminhamento ativo' })
      .click();
    const active = page.getByRole('dialog', { name: 'Encaminhamentos ativos' });
    const item = active.getByRole('listitem', { name: '127.0.0.1:17010 → echo:7000' });
    await expect(item).toContainText('pw-host');
    expect(await echo(17010, 'ainda ativo')).toBe('ainda ativo');
    await item.getByRole('button', { name: 'Desativar' }).click();
    await expect(item).toHaveCount(0);
  } finally {
    await app.close();
  }
});
