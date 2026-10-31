// Layout estável ao maximizar, restaurar e redimensionar em várias escalas (T043; regressão do
// bug 3: faixas vazias, terminal com altura 0 e PTY com tamanho diferente do xterm).
import { expect, test, type Page } from '@playwright/test';
import { launchApp, testService } from './helpers/launch';
import {
  createVault,
  openLocalTab,
  resizeCount,
  tab,
  terminalPanel,
  terminalSize,
  waitPowerShellPrompt,
} from './helpers/ui';

interface Size {
  cols: number;
  rows: number;
}

/** Os quatro cantos da viewport pertencem ao app (sem faixa vazia em volta). */
async function coversViewport(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const root = document.getElementById('root');
    const w = window.innerWidth;
    const h = window.innerHeight;
    const points: [number, number][] = [
      [1, 1],
      [w - 2, 1],
      [1, h - 2],
      [w - 2, h - 2],
    ];
    return points.every(([x, y]) => {
      const el = document.elementFromPoint(x, y);
      return el !== null && root?.contains(el) === true && el !== root;
    });
  });
}

/** O canvas do xterm ocupa o contêiner: sobra no máximo uma célula de cada lado. */
async function terminalFills(page: Page, title: string): Promise<boolean> {
  return terminalPanel(page, title)
    .locator('.xterm')
    .evaluate((xterm) => {
      const container = xterm.parentElement;
      const screen = xterm.querySelector('.xterm-screen');
      const row = xterm.querySelector('.xterm-rows > div');
      if (!container || !screen) return false;
      const c = container.getBoundingClientRect();
      const s = screen.getBoundingClientRect();
      const cell = row?.getBoundingClientRect().height ?? 24;
      return (
        c.height > 0 &&
        s.height > 0 &&
        s.width <= c.width + 1 &&
        s.height <= c.height + 1 &&
        c.width - s.width < 40 &&
        c.height - s.height < cell + 24
      );
    });
}

async function expectSettled(page: Page, title: string, sessionId: string) {
  await expect.poll(() => coversViewport(page)).toBe(true);
  await expect.poll(() => terminalFills(page, title)).toBe(true);
  await expect
    .poll(async () => {
      const client = await terminalSize(page, sessionId);
      const server = await testService<Size | null>(page, 'ServerSizeOf', sessionId);
      return client !== null && server?.cols === client.cols && server.rows === client.rows;
    })
    .toBe(true);
}

for (const scale of ['1', '1.25', '1.5']) {
  test(`escala ${scale}: 20 ciclos de maximizar, restaurar e redimensionar`, async () => {
    test.setTimeout(240_000);
    const app = await launchApp({ env: { OPENTERMINAL_E2E_SCALE: scale } });
    const { page } = app;
    try {
      expect(await page.evaluate(() => window.devicePixelRatio)).toBe(Number(scale));
      await createVault(page);
      const cmd = await openLocalTab(page, 'Prompt de Comando');
      const ps = await openLocalTab(page, 'PowerShell');
      await waitPowerShellPrompt(page, ps);

      const sizes = [
        [800, 500],
        [1100, 700],
        [960, 620],
        [1280, 800],
      ] as const;
      for (let i = 0; i < 20; i++) {
        await testService(page, 'Maximise');
        await expectSettled(page, 'PowerShell', ps);
        await testService(page, 'Restore');
        await expectSettled(page, 'PowerShell', ps);
        const [w, h] = sizes[i % sizes.length] ?? sizes[0];
        await testService(page, 'SetSize', w, h);
        await expectSettled(page, 'PowerShell', ps);
      }

      // A aba que ficou oculta acerta o tamanho ao voltar a ser exibida.
      await tab(page, 'Prompt de Comando').click();
      await expectSettled(page, 'Prompt de Comando', cmd);

      // Arrastar a borda: muitos tamanhos seguidos viram poucos resizes no servidor.
      const before = await resizeCount(page, cmd);
      for (let step = 0; step < 30; step++) {
        await testService(page, 'SetSize', 900 + step * 10, 600 + step * 5);
      }
      await expectSettled(page, 'Prompt de Comando', cmd);
      expect((await resizeCount(page, cmd)) - before).toBeLessThanOrEqual(5);
    } finally {
      await app.close();
    }
  });
}
