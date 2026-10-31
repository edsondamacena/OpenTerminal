// Menus ancorados e inteiros na janela (T044; regressão do bug 4, FR-132) e foco de diálogos
// (regressões B2 e B3).
import { expect, test, type Locator, type Page } from '@playwright/test';
import { SSHD, up } from '../../../scripts/sshd.mjs';
import { launchApp, testService } from './helpers/launch';
import {
  acceptHostKey,
  addHost,
  createVault,
  expectTerminalLine,
  hostItem,
  openLocalTab,
  sessionIdOf,
  waitConnected,
} from './helpers/ui';

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

async function boxOf(locator: Locator): Promise<Box> {
  const box = await locator.boundingBox();
  if (!box) throw new Error('elemento sem caixa');
  return box;
}

async function expectInsideViewport(page: Page, menu: Locator) {
  const box = await boxOf(menu);
  const { width, height } = await page.evaluate(() => ({
    width: window.innerWidth,
    height: window.innerHeight,
  }));
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(width);
  expect(box.y + box.height).toBeLessThanOrEqual(height);
}

/** O menu está por cima de tudo no seu centro (não fica atrás do terminal). */
async function expectOnTop(page: Page, menu: Locator) {
  const box = await boxOf(menu);
  const onTop = await menu.evaluate((el, p) => el.contains(document.elementFromPoint(p.x, p.y)), {
    x: box.x + box.width / 2,
    y: box.y + box.height / 2,
  });
  expect(onTop).toBe(true);
}

test('menu "+" no tamanho mínimo abre colado ao botão, inteiro e acima do terminal', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await openLocalTab(page, 'PowerShell');
    await testService(page, 'SetSize', 800, 500);
    await expect.poll(() => page.evaluate(() => window.innerWidth)).toBeLessThanOrEqual(800);

    const plus = page.getByRole('button', { name: 'Novo terminal local' });
    await plus.click();
    const menu = page.getByRole('menu');
    await expect(menu).toBeVisible();

    const button = await boxOf(plus);
    const box = await boxOf(menu);
    expect(box.y - (button.y + button.height)).toBeGreaterThanOrEqual(0);
    expect(box.y - (button.y + button.height)).toBeLessThanOrEqual(8);
    await expectInsideViewport(page, menu);
    await expectOnTop(page, menu);
    await expect(page.getByRole('menuitem').first()).toBeFocused();

    // Clicar de novo no botão fecha; Esc também fecha e devolve o foco ao botão.
    await plus.click();
    await expect(menu).toBeHidden();
    await plus.click();
    await expect(menu).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(menu).toBeHidden();
    await expect(plus).toBeFocused();
  } finally {
    await app.close();
  }
});

test('menu de contexto de host aberto perto dos quatro cantos fica inteiro', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await addHost(page, {
      label: 'acme-web',
      address: '10.1.2.3',
      port: 22,
      username: 'deploy',
      method: 'Perguntar ao conectar',
    });
    const row = hostItem(page, 'acme-web');
    const { width, height } = await page.evaluate(() => ({
      width: window.innerWidth,
      height: window.innerHeight,
    }));
    const corners = [
      { clientX: 2, clientY: 2 },
      { clientX: width - 2, clientY: 2 },
      { clientX: 2, clientY: height - 2 },
      { clientX: width - 2, clientY: height - 2 },
    ];
    for (const point of corners) {
      await row.dispatchEvent('contextmenu', { ...point, bubbles: true, cancelable: true });
      const menu = page.getByRole('menu');
      await expect(menu).toBeVisible();
      await expectInsideViewport(page, menu);
      await expectOnTop(page, menu);
      await page.keyboard.press('Escape');
      await expect(menu).toBeHidden();
    }
  } finally {
    await app.close();
  }
});

test('Esc fecha o diálogo de encaminhamento depois de o botão focado sumir (B2)', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await addHost(page, {
      label: 'acme-web',
      address: '10.1.2.3',
      port: 22,
      username: 'deploy',
      method: 'Perguntar ao conectar',
    });
    await hostItem(page, 'acme-web').click({ button: 'right' });
    await page.getByRole('menuitem', { name: 'Encaminhamento de portas' }).click();
    const dialog = page.getByRole('dialog', { name: /Encaminhamento de portas/ });
    await expect(dialog).toBeVisible();

    // "Nova regra" some ao abrir o formulário; "Cancelar" some ao fechá-lo.
    await dialog.getByRole('button', { name: 'Nova regra' }).click();
    await dialog.getByRole('button', { name: 'Cancelar' }).click();
    await expect(dialog.getByRole('button', { name: 'Nova regra' })).toBeVisible();
    expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  } finally {
    await app.close();
  }
});

test('depois de aceitar a host key, digitar vai para o terminal sem clicar (B3)', async () => {
  await up();
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
    await hostItem(page, 'pw-host').dblclick();
    await acceptHostKey(page);
    await waitConnected(page, 'pw-host');
    const id = await sessionIdOf(page, 'pw-host');
    await page.keyboard.type('echo foco-ok\n');
    await expectTerminalLine(page, id, 'foco-ok');
  } finally {
    await app.close();
  }
});
