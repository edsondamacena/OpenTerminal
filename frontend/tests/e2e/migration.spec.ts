// US1 — cofre da 001 aberto e migrado pela versão nova (T042, FR-102, SC-114).
import { cpSync, mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';
import { up } from '../../../scripts/sshd.mjs';
import { launchApp } from './helpers/launch';
import { REPO_ROOT } from './helpers/paths';
import { groupItem, hostItem, sidebarHosts, waitConnected } from './helpers/ui';

const FIXTURE = join(REPO_ROOT, 'tests', 'fixtures', 'vault-001');

interface Expected {
  password: string;
  content: {
    groups: { name: string }[];
    hosts: { label: string }[];
    credentials: { name: string }[];
  };
}

const expected = JSON.parse(readFileSync(join(FIXTURE, 'expected.json'), 'utf8')) as Expected;

// O host "Sshd de teste" da fixture aponta para o OpenSSH de teste com host key fixa.
test.beforeAll(async () => {
  await up();
});

test('desbloquear o cofre da 001 migra tudo e conecta sem reconfirmar a host key', async () => {
  const dataDir = mkdtempSync(join(tmpdir(), 'openterminal-e2e-001-'));
  cpSync(FIXTURE, dataDir, { recursive: true, filter: (src) => !src.endsWith('expected.json') });
  const app = await launchApp({ dataDir });
  const { page } = app;
  try {
    const form = page.getByRole('form', { name: 'Atualizar cofre' });
    await expect(form).toContainText('Encontramos o cofre da versão anterior.');

    // Senha errada não migra e mantém o estado.
    await form.getByLabel('Senha mestra', { exact: true }).fill('senha-errada');
    await form.getByRole('button', { name: 'Atualizar cofre' }).click();
    await expect(form.getByRole('alert')).toHaveText('Senha incorreta ou cofre inválido.');

    await form.getByLabel('Senha mestra', { exact: true }).fill(expected.password);
    await form.getByRole('button', { name: 'Atualizar cofre' }).click();
    await expect(sidebarHosts(page)).toBeVisible({ timeout: 30_000 });

    for (const group of expected.content.groups)
      await expect(groupItem(page, group.name)).toBeVisible();
    for (const host of expected.content.hosts)
      await expect(hostItem(page, host.label)).toBeVisible();

    await page.getByRole('button', { name: 'Credenciais e chaves…' }).first().click();
    const credentials = page.getByRole('dialog', { name: 'Credenciais e chaves' });
    for (const credential of expected.content.credentials) {
      await expect(
        credentials.getByRole('listitem').filter({ hasText: credential.name }),
      ).toBeVisible();
    }
    await credentials.getByRole('button', { name: 'Fechar' }).last().click();
    await expect(credentials).toBeHidden();

    // Host key aceita na 001: conecta direto, sem o diálogo de confirmação.
    await hostItem(page, 'Sshd de teste').dblclick();
    await waitConnected(page, 'Sshd de teste');
    await expect(page.getByRole('dialog', { name: 'Confirme a chave do servidor' })).toHaveCount(0);
  } finally {
    await app.close();
  }

  // Segunda execução: o cofre já migrado abre pelo desbloqueio normal.
  const again = await launchApp({ dataDir });
  try {
    await expect(again.page.getByRole('form', { name: 'Cofre bloqueado' })).toBeVisible();
  } finally {
    await again.close();
  }
});
