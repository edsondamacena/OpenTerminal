// Uso diário (FR-173–FR-177, T125): atalhos que não roubam teclas do shell, remapeamento que
// persiste, erro com código e detalhes, $TERM escolhido e opções de comportamento do terminal.
import { resolve } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import * as sshd from '../../../scripts/sshd.mjs';
import { launchApp, testService } from './helpers/launch';
import { REPO_ROOT } from './helpers/paths';
import {
  acceptHostKey,
  addHost,
  createVault,
  expectTerminalLine,
  hostItem,
  selectInTerminal,
  sessionIdOf,
  tab,
  terminalPanel,
  terminalText,
  typeInTerminal,
  unlock,
  waitConnected,
} from './helpers/ui';

const { SSHD } = sshd;

async function addPwHost(page: Page) {
  await addHost(page, {
    label: 'pw-host',
    address: SSHD.host,
    port: SSHD.port,
    username: 'pwuser',
    password: 'pw-test-123',
  });
}

async function openPwHost(page: Page): Promise<string> {
  await hostItem(page, 'pw-host').dblclick();
  await acceptHostKey(page);
  await waitConnected(page, 'pw-host');
  return sessionIdOf(page, 'pw-host');
}

async function openSettings(page: Page, section: string) {
  await page.getByRole('button', { name: 'Configurações' }).first().click();
  const settings = page.getByRole('region', { name: 'Configurações' });
  await settings.getByRole('button', { name: section, exact: true }).click();
  return settings;
}

/** Fecha a aba conectada, confirmando se o app perguntar. */
async function confirmCloseIfAsked(page: Page) {
  const confirm = page.getByRole('dialog').getByRole('button', { name: /Fechar/ });
  if (await confirm.isVisible().catch(() => false)) await confirm.click();
}

/** Troca a área de transferência do WebView por uma falsa, para ler e escrever sem permissão. */
async function fakeClipboard(page: Page, toPaste: string) {
  await page.evaluate((text) => {
    const w = window as unknown as { __copied: string[] };
    w.__copied = [];
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: (value: string) => {
          w.__copied.push(value);
          return Promise.resolve();
        },
        readText: () => Promise.resolve(text),
      },
    });
  }, toPaste);
}

const copied = (page: Page) =>
  page.evaluate(() => (window as unknown as { __copied: string[] }).__copied);

test('Ctrl+W pede confirmação e fecha a aba; $TERM escolhido chega ao servidor', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await addPwHost(page);

    // $TERM por seleção na seção Conexão vale para a próxima conexão.
    const settings = await openSettings(page, 'Conexão (SSH)');
    await settings.getByLabel('Tipo de terminal ($TERM)').click();
    await page.getByRole('option', { name: 'vt100' }).click();
    await page.keyboard.press('Escape');
    await expect(settings).toBeHidden();

    const id = await openPwHost(page);
    await typeInTerminal(page, 'pw-host', 'echo "term=$TERM"\r');
    await expectTerminalLine(page, id, 'term=vt100');

    // Ctrl+W pergunta; cancelar mantém a aba, confirmar fecha.
    await page.keyboard.press('Control+W');
    await page.getByRole('button', { name: 'Cancelar' }).click();
    await expect(tab(page, 'pw-host')).toBeVisible();

    await page.keyboard.press('Control+W');
    await page.getByRole('button', { name: 'Fechar aba' }).click();
    await expect(tab(page, 'pw-host')).toHaveCount(0);
  } finally {
    await app.close();
  }
});

test('remapear e desativar atalhos persiste após reiniciar', async () => {
  let app = await launchApp();
  const { dataDir } = app;
  try {
    await createVault(app.page);
    let settings = await openSettings(app.page, 'Atalhos');
    await settings.getByRole('button', { name: 'Trocar o atalho de "Fechar a aba"' }).click();
    await app.page.keyboard.press('Control+Alt+W');
    // Ctrl+Alt+letra também é tecla do terminal: o editor avisa e pede confirmação.
    await settings.getByRole('button', { name: 'Usar mesmo assim' }).click();
    await expect(
      settings.getByRole('button', { name: 'Trocar o atalho de "Fechar a aba"' }),
    ).toContainText('Ctrl+Alt+W');
    await settings.getByRole('button', { name: 'Desativar: Paleta de comandos' }).click();
    await expect(
      settings.getByRole('button', { name: 'Trocar o atalho de "Paleta de comandos"' }),
    ).toContainText('Desativado');
    await app.page.waitForTimeout(500);
    await app.close();

    app = await launchApp({ dataDir });
    const { page } = app;
    await unlock(page);
    settings = await openSettings(page, 'Atalhos');
    await expect(
      settings.getByRole('button', { name: 'Trocar o atalho de "Fechar a aba"' }),
    ).toContainText('Ctrl+Alt+W');
    await expect(
      settings.getByRole('button', { name: 'Trocar o atalho de "Paleta de comandos"' }),
    ).toContainText('Desativado');
    await page.keyboard.press('Escape');

    await page.keyboard.press('Control+Shift+T');
    await expect(page.getByRole('tab')).toHaveCount(1);
    // O atalho antigo não fecha mais; o novo fecha. A paleta desativada não abre.
    await page.keyboard.press('Control+Shift+W');
    await page.keyboard.press('Control+Shift+P');
    await expect(page.getByRole('dialog', { name: 'Paleta de comandos' })).toHaveCount(0);
    await expect(page.getByRole('tab')).toHaveCount(1);
    await page.keyboard.press('Control+Alt+W');
    await expect(page.getByRole('tab')).toHaveCount(0);
  } finally {
    await app.close();
  }
});

test('erro mostra o código e copia os detalhes', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await fakeClipboard(page, '');
    const settings = await openSettings(page, 'Terminal');
    await testService(
      page,
      'SetDialogPath',
      resolve(REPO_ROOT, 'tests/fixtures/schemes/invalid.json'),
    );
    await settings.getByRole('button', { name: 'Importar…' }).click();
    const toast = page.getByRole('alert').filter({ hasText: 'SCHEME_INVALID' });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Copiar detalhes' }).click();
    await expect(toast.getByRole('button', { name: 'Copiado' })).toBeVisible();
    const [details] = await copied(page);
    expect(details).toContain('SCHEME_INVALID');
    expect(details).toContain('ProfileService.ImportScheme');
  } finally {
    await app.close();
  }
});

test('copiar ao selecionar, colar com o botão direito e reconexão automática', async () => {
  test.setTimeout(180_000);
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await addPwHost(page);
    const id = await openPwHost(page);
    await fakeClipboard(page, 'echo colado-pelo-botao');

    // Padrão: selecionar não copia; colar com o botão direito está ligado.
    await typeInTerminal(page, 'pw-host', 'echo marca-selecao\r');
    await expectTerminalLine(page, id, 'marca-selecao');
    await selectInTerminal(page, id, 'marca-selecao');
    await page.waitForTimeout(400);
    expect(await copied(page)).toEqual([]);

    await terminalPanel(page, 'pw-host').locator('.xterm').click({ button: 'right' });
    await page.keyboard.press('Enter');
    await expectTerminalLine(page, id, 'colado-pelo-botao');

    // Liga copiar ao selecionar, desliga colar no botão direito e liga a reconexão automática.
    const settings = await openSettings(page, 'Terminal');
    await settings.getByLabel('Copiar ao selecionar').check();
    await settings.getByLabel('Colar com o botão direito').uncheck();
    await settings.getByLabel('Reconectar automaticamente').check();
    await page.keyboard.press('Escape');
    await expect(settings).toBeHidden();

    // Seleção nova (a mesma de antes não dispara mudança de seleção).
    await typeInTerminal(page, 'pw-host', 'echo outra-selecao\r');
    await expectTerminalLine(page, id, 'outra-selecao');
    await selectInTerminal(page, id, 'outra-selecao');
    await expect.poll(() => copied(page)).toContain('outra-selecao');

    const before = (await terminalText(page, id)).split('colado-pelo-botao').length;
    await terminalPanel(page, 'pw-host').locator('.xterm').click({ button: 'right' });
    await page.keyboard.press('Enter');
    await page.waitForTimeout(800);
    expect((await terminalText(page, id)).split('colado-pelo-botao').length).toBe(before);

    // Queda do servidor: a faixa mostra a contagem e a aba volta sozinha quando ele sobe.
    sshd.stop();
    await expect(terminalPanel(page, 'pw-host').getByRole('status')).toContainText(
      'Reconectando em',
      { timeout: 60_000 },
    );
    await sshd.start();
    await waitConnected(page, 'pw-host');
    expect(await sessionIdOf(page, 'pw-host')).toBe(id);
    await typeInTerminal(page, 'pw-host', 'echo voltou\r');
    await expectTerminalLine(page, id, 'voltou');
  } finally {
    await sshd.start().catch(() => undefined);
    await app.close();
  }
});
