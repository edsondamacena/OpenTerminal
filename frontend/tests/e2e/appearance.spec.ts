// Aparência nos cinco modos (US5, T087): contraste dos textos principais, superfícies opacas sem
// vidro, terminal opaco e capturas de cada tela em test-results/brand/ para revisão.
import { mkdirSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { launchApp } from './helpers/launch';
import { addHost, createVault, openLocalTab } from './helpers/ui';

const OUT = 'test-results/brand';

interface Mode {
  name: 'glass-dark' | 'glass-light' | 'solid-dark' | 'solid-light' | 'high-contrast';
  glass: boolean;
  dark: boolean;
  highContrast: boolean;
}

const MODES: Mode[] = [
  { name: 'solid-dark', glass: false, dark: true, highContrast: false },
  { name: 'solid-light', glass: false, dark: false, highContrast: false },
  { name: 'glass-dark', glass: true, dark: true, highContrast: false },
  { name: 'glass-light', glass: true, dark: false, highContrast: false },
  { name: 'high-contrast', glass: false, dark: true, highContrast: true },
];

/** Textos que precisam de 4,5:1: título da lista, host, aba, barra de status e diálogo. */
const TEXTS = [
  'h2',
  '[role="treeitem"] span',
  '[role="tab"]',
  '[role="dialog"] h2',
  '[role="dialog"] label',
];

async function setMode(page: Page, mode: Mode): Promise<void> {
  await page.evaluate((m) => {
    const w = window as unknown as {
      __setAppearance: (a: { glass: boolean; dark: boolean; highContrast: boolean }) => void;
    };
    w.__setAppearance({ glass: m.glass, dark: m.dark, highContrast: m.highContrast });
  }, mode);
  await expect(page.locator('html')).toHaveAttribute('data-mode', mode.name);
}

/**
 * Menor contraste entre o texto e o fundo efetivo (camadas translúcidas compostas até a raiz;
 * no vidro, o que fica por trás é o pior caso: preto no escuro, branco no claro).
 */
async function minContrast(page: Page, dark: boolean): Promise<{ ratio: number; where: string }> {
  return page.evaluate(
    ({ selectors, dark }) => {
      const parse = (c: string): [number, number, number, number] => {
        const m = c.match(/rgba?\(([^)]+)\)/);
        if (!m?.[1]) return [0, 0, 0, 0];
        const p = m[1]
          .split(/[ ,/]+/)
          .filter(Boolean)
          .map(Number);
        return [p[0] ?? 0, p[1] ?? 0, p[2] ?? 0, p[3] ?? 1];
      };
      const over = (top: number[], bottom: number[]) => {
        const a = top[3] ?? 1;
        return [0, 1, 2].map((i) => (top[i] ?? 0) * a + (bottom[i] ?? 0) * (1 - a));
      };
      const lum = (rgb: number[]) => {
        const [r, g, b] = rgb.map((v) => {
          const s = v / 255;
          return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
        });
        return 0.2126 * (r ?? 0) + 0.7152 * (g ?? 0) + 0.0722 * (b ?? 0);
      };
      const background = (el: Element): number[] => {
        const layers: number[][] = [];
        for (let n: Element | null = el; n; n = n.parentElement) {
          const bg = parse(getComputedStyle(n).backgroundColor);
          if (bg[3] > 0) layers.push(bg);
          if (bg[3] >= 1) break;
        }
        let rgb = dark ? [0, 0, 0] : [255, 255, 255];
        for (const layer of layers.reverse()) rgb = over(layer, rgb);
        return rgb;
      };
      let worst = { ratio: Infinity, where: '' };
      for (const selector of selectors) {
        for (const el of document.querySelectorAll(selector)) {
          // Decorativos (avatar com iniciais sobre degradê) ficam fora: aria-hidden.
          if (
            !(el instanceof HTMLElement) ||
            el.offsetParent === null ||
            el.closest('[aria-hidden="true"]') !== null ||
            !el.textContent.trim()
          )
            continue;
          const fg = parse(getComputedStyle(el).color);
          const bg = background(el);
          const text = over(fg, bg);
          const [a, b] = [lum(text), lum(bg)];
          const ratio = (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
          if (ratio < worst.ratio)
            worst = { ratio, where: `${selector}: ${el.textContent.trim()}` };
        }
      }
      return worst;
    },
    { selectors: TEXTS, dark },
  );
}

test('cinco modos: contraste, superfícies opacas sem vidro e terminal opaco (FR-160–FR-165)', async () => {
  test.setTimeout(180_000);
  mkdirSync(OUT, { recursive: true });
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await addHost(page, {
      label: 'core-router-01',
      address: '10.0.0.1',
      port: 22,
      username: 'noc',
      method: 'Perguntar ao conectar',
    });
    await openLocalTab(page, 'PowerShell');

    for (const mode of MODES) {
      await setMode(page, mode);
      await page.screenshot({ path: `${OUT}/${mode.name}-terminal.png` });

      // Sem vidro, nenhuma superfície da moldura deixa a área de trabalho aparecer.
      if (!mode.glass) {
        const translucent = await page.evaluate(() =>
          ['body', '#root', '[data-terminal-area]']
            .map((s) => document.querySelector(s))
            .filter((el): el is Element => el !== null)
            .some((el) => {
              const bg = getComputedStyle(el).backgroundColor;
              return bg !== 'rgba(0, 0, 0, 0)' && /rgba\([^)]*, 0?\.\d+\)/.test(bg);
            }),
        );
        expect(translucent, `${mode.name}: superfície translúcida sem vidro`).toBe(false);
      }

      // Terminal 100% opaco por padrão, em todos os modos.
      const terminalAlpha = await page.evaluate(() => {
        const el = document.querySelector('.xterm-viewport') ?? document.querySelector('.xterm');
        const bg = el ? getComputedStyle(el).backgroundColor : '';
        const m = bg.match(/rgba\([^)]*,\s*([\d.]+)\)/);
        return m ? Number(m[1]) : 1;
      });
      expect(terminalAlpha, `${mode.name}: terminal translúcido`).toBe(1);

      await page.getByRole('button', { name: 'Novo host' }).click();
      await expect(page.getByRole('dialog', { name: 'Novo host' })).toBeVisible();
      await page.screenshot({ path: `${OUT}/${mode.name}-dialogo.png` });
      const worst = await minContrast(page, mode.dark);
      expect(worst.ratio, `${mode.name}: ${worst.where}`).toBeGreaterThanOrEqual(4.5);
      await page.keyboard.press('Escape');
      await expect(page.getByRole('dialog', { name: 'Novo host' })).toBeHidden();
    }
  } finally {
    await app.close();
  }
});
