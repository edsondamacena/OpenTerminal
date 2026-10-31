// Acabamento da interface (US5, T114): barra lateral recolhível, configurações como página e
// barra de status.
import { expect, test, type Page } from '@playwright/test';
import { launchApp, testService } from './helpers/launch';
import {
  createVault,
  openLocalTab,
  resizeCount,
  sidebarHosts,
  terminalSize,
  unlock,
  waitPowerShellPrompt,
} from './helpers/ui';

interface Size {
  cols: number;
  rows: number;
}

const rail = (page: Page) => page.getByRole('navigation', { name: 'Barra lateral recolhida' });

async function expectServerMatches(page: Page, sessionId: string): Promise<void> {
  await expect
    .poll(async () => {
      const [xterm, server] = await Promise.all([
        terminalSize(page, sessionId),
        testService<Size | null>(page, 'ServerSizeOf', sessionId),
      ]);
      return JSON.stringify(server) === JSON.stringify(xterm);
    })
    .toBe(true);
}

test('Ctrl+Shift+B recolhe e expande a lateral sem reconectar e o estado volta após reiniciar (FR-167)', async () => {
  let app = await launchApp();
  const { dataDir } = app;
  try {
    const { page } = app;
    await createVault(page);
    const id = await openLocalTab(page, 'PowerShell');
    await waitPowerShellPrompt(page, id);
    const before = await terminalSize(page, id);
    if (!before) throw new Error('terminal sem tamanho');

    await page.keyboard.press('Control+Shift+B');
    await expect(rail(page)).toBeVisible();
    await expect(sidebarHosts(page)).toBeHidden();
    await expect
      .poll(async () => (await terminalSize(page, id))?.cols)
      .toBeGreaterThan(before.cols);
    await expectServerMatches(page, id);
    // A mesma sessão continua: o painel ainda é o da aba aberta antes.
    await expect(page.locator(`[data-session-id="${id}"]`)).toBeVisible();

    // Rótulo do ícone aparece ao passar o mouse.
    await rail(page).getByRole('button', { name: 'Encaminhamentos ativos' }).hover();
    await expect(page.getByRole('tooltip')).toHaveText('Encaminhamentos ativos');

    await page.keyboard.press('Control+Shift+B');
    await expect(sidebarHosts(page)).toBeVisible();
    await expect(rail(page)).toBeHidden();
    await expect.poll(async () => (await terminalSize(page, id))?.cols).toBe(before.cols);
    await expectServerMatches(page, id);
    // Uma ida e volta não deve gerar uma enxurrada de resize no servidor.
    expect(await resizeCount(page, id)).toBeLessThanOrEqual(6);

    // Recolhida, a escolha persiste entre execuções.
    await page.getByRole('button', { name: 'Recolher barra lateral (Ctrl+Shift+B)' }).click();
    await expect(rail(page)).toBeVisible();
    await app.close();

    app = await launchApp({ dataDir });
    await expect(rail(app.page)).toBeVisible({ timeout: 30_000 });
    await rail(app.page).getByRole('button', { name: 'Desbloquear o cofre' }).click();
    await unlock(app.page);
    await expect(sidebarHosts(app.page)).toBeVisible();
  } finally {
    await app.close();
  }
});

test('configurações abrem como página, Esc volta à aba e a barra de status mostra a aba ativa (FR-168, FR-172)', async () => {
  const app = await launchApp();
  try {
    const { page } = app;
    await createVault(page);
    const id = await openLocalTab(page, 'PowerShell');
    await waitPowerShellPrompt(page, id);

    const status = page.getByRole('contentinfo', { name: 'Barra de status' });
    await expect(status.getByTestId('status-tab')).toContainText('PowerShell');
    await expect(status.getByTestId('status-state')).toHaveText('Em execução');

    await page.keyboard.press('Control+Shift+P');
    await page.keyboard.type('configura');
    await page.keyboard.press('Enter');
    const settings = page.getByRole('region', { name: 'Configurações' });
    await expect(settings).toBeVisible();
    // A aba segue aberta por baixo, sem reconectar.
    await expect(page.getByRole('tab', { name: 'PowerShell' })).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(settings).toBeHidden();
    await expect(page.locator(`[data-session-id="${id}"]`)).toBeVisible();

    // Clicar numa aba também volta ao terminal.
    await page.getByRole('button', { name: 'Configurações' }).click();
    await expect(settings).toBeVisible();
    await page.getByRole('tab', { name: 'PowerShell' }).click();
    await expect(settings).toBeHidden();
  } finally {
    await app.close();
  }
});

test('trocar a cor de destaque muda --accent e mantém 4,5:1 (FR-171)', async () => {
  const app = await launchApp();
  try {
    const { page } = app;
    await createVault(page);
    const accent = () =>
      page.evaluate(() =>
        getComputedStyle(document.documentElement).getPropertyValue('--accent').trim(),
      );
    const laranja = await accent();
    // Laranja da marca: claro no tema escuro, profundo no claro (segue o Windows).
    expect(['#fd7b17', '#b04e06']).toContain(laranja.toLowerCase());

    await page.getByRole('button', { name: 'Configurações' }).first().click();
    const settings = page.getByRole('region', { name: 'Configurações' });
    await settings.getByRole('button', { name: 'Aparência', exact: true }).click();
    await settings.getByLabel('Cor de destaque').click();
    await page.getByRole('option', { name: 'Grafite' }).click();
    await expect.poll(accent).not.toBe(laranja);

    // Texto do botão primário sobre o destaque e o destaque sobre o fundo da página.
    const ratios = await page.evaluate(() => {
      const css = getComputedStyle(document.documentElement);
      const hex = (name: string) => css.getPropertyValue(name).trim();
      const lum = (h: string) => {
        const [r, g, b] = [1, 3, 5].map((i) => {
          const v = parseInt(h.slice(i, i + 2), 16) / 255;
          return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
        });
        return 0.2126 * (r ?? 0) + 0.7152 * (g ?? 0) + 0.0722 * (b ?? 0);
      };
      const ratio = (a: string, b: string) => {
        const [x, y] = [lum(a), lum(b)];
        return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
      };
      return { text: ratio(hex('--accent-text'), hex('--accent')) };
    });
    expect(ratios.text).toBeGreaterThanOrEqual(4.5);
  } finally {
    await app.close();
  }
});

test('com "reduzir movimento" as durações computadas são 0 (FR-170)', async () => {
  const app = await launchApp({ motion: true });
  try {
    const { page } = app;
    await createVault(page);
    await openLocalTab(page, 'PowerShell');
    const durations = () =>
      page.evaluate(() =>
        ['[role="tab"]', '[role="tablist"]'].map((s) => {
          const el = document.querySelector(s);
          if (!el) return 'ausente';
          const st = getComputedStyle(el);
          const before = getComputedStyle(el, '::before');
          return `${st.transitionDuration}|${before.transitionDuration}`;
        }),
      );
    expect((await durations()).join()).toMatch(/[1-9]/);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    for (const d of await durations()) expect(d).toMatch(/^0s(, 0s)*\|0s(, 0s)*$/);
  } finally {
    await app.close();
  }
});
