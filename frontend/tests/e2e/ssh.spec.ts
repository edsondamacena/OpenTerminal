// US2 — hosts e sessões SSH em abas (T041; da 001 T057).
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';
import { KEYS_DIR } from '../../../scripts/gen-test-keys.mjs';
import { SSHD, start, stop, up } from '../../../scripts/sshd.mjs';
import { launchApp } from './helpers/launch';
import {
  acceptHostKey,
  addHost,
  createVault,
  expectTerminalLine,
  hostItem,
  importKey,
  sessionIdOf,
  tab,
  terminalPanel,
  typeInTerminal,
  unlockForm,
  waitConnected,
} from './helpers/ui';

// Mesmo OpenSSH de teste da integração (tests/fixtures/sshd).
test.beforeAll(async () => {
  await up();
});

test('conectar por senha e chave em abas, bloquear o cofre e perder a conexão', async () => {
  const app = await launchApp();
  const { page } = app;
  let stopped = false;
  try {
    await createVault(page);

    await addHost(page, {
      label: 'pw-host',
      address: SSHD.host,
      port: SSHD.port,
      username: 'pwuser',
      password: 'pw-test-123',
    });
    await hostItem(page, 'pw-host').dblclick();
    await acceptHostKey(page);
    await waitConnected(page, 'pw-host');
    const pwSession = await sessionIdOf(page, 'pw-host');
    await typeInTerminal(page, 'pw-host', 'echo ok\n');
    await expectTerminalLine(page, pwSession, 'ok');

    // Segunda aba com chave Ed25519; a host key já aceita não pergunta de novo.
    await importKey(page, 'chave-ed25519', readFileSync(join(KEYS_DIR, 'ed25519'), 'utf8'));
    await addHost(page, {
      label: 'key-host',
      address: SSHD.host,
      port: SSHD.port,
      username: 'keyuser',
      method: 'Chave privada',
      credential: 'chave-ed25519',
    });
    await hostItem(page, 'key-host').dblclick();
    await waitConnected(page, 'key-host');
    const keySession = await sessionIdOf(page, 'key-host');
    await typeInTerminal(page, 'key-host', 'whoami\n');
    await expectTerminalLine(page, keySession, 'keyuser');

    // Barra de status mostra a aba ativa e o estado (FR-172).
    const status = page.getByRole('contentinfo', { name: 'Barra de status' });
    await expect(status.getByTestId('status-tab')).toContainText('key-host');
    await expect(status.getByTestId('status-tab')).toContainText(`keyuser@${SSHD.host}`);
    await expect(status.getByTestId('status-state')).toHaveText('Conectado');

    // Alternar abas mantém o conteúdo (US2-5).
    await tab(page, 'pw-host').click();
    await expect(terminalPanel(page, 'pw-host')).toBeVisible();
    await expect(terminalPanel(page, 'key-host')).toBeHidden();
    await expectTerminalLine(page, pwSession, 'ok');
    await tab(page, 'key-host').click();
    await expectTerminalLine(page, keySession, 'keyuser');

    // Bloquear o cofre não fecha sessões (FR-003).
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await expect(unlockForm(page)).toBeVisible();
    await expect(tab(page, 'pw-host')).toBeVisible();
    await tab(page, 'pw-host').click();
    await typeInTerminal(page, 'pw-host', 'echo still-here\n');
    await expectTerminalLine(page, pwSession, 'still-here');

    // Servidor cai → aba desconectada com botão Reconectar (US2-6).
    stop();
    stopped = true;
    const panel = terminalPanel(page, 'pw-host');
    await expect(panel.getByRole('status')).toContainText('Desconectado', { timeout: 60_000 });
    await expect(panel.getByRole('button', { name: 'Reconectar' })).toBeVisible();
  } finally {
    await app.close();
    if (stopped) await start();
  }
});
