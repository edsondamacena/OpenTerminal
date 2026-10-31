// SC-112 — troca de aba < 100 ms com 20 abas (uma com saída contínua) e busca em 1.000 hosts < 200 ms (T099).
import { expect, test, type Page } from '@playwright/test';
import { launchApp } from './helpers/launch';
import { createVault, hostItem, unlock, unlockForm } from './helpers/ui';

const TABS = 20;
const HOSTS = 1_000;
const SWITCH_MS = 100;
const SEARCH_MS = 200;

/** Clica na aba `index` e mede até o painel dela estar visível e o quadro seguinte pintado. */
async function switchTime(page: Page, index: number): Promise<number> {
  return page.evaluate(async (i) => {
    const frame = () => new Promise<number>((r) => requestAnimationFrame(r));
    const tabs = [...document.querySelectorAll<HTMLElement>('[role=tab]')];
    const panels = [...document.querySelectorAll<HTMLElement>('[role=tabpanel][data-session-id]')];
    const target = tabs[i];
    // Abas recém-abertas: os painéis seguem a ordem das abas.
    const panel = panels[i];
    if (!target) throw new Error(`aba ${String(i)} não existe`);
    const t0 = performance.now();
    target.click();
    for (let n = 0; n < 120; n++) {
      await frame();
      const shown = panel
        ? panel.checkVisibility()
        : target.getAttribute('aria-selected') === 'true';
      if (shown && target.getAttribute('aria-selected') === 'true') break;
    }
    await frame();
    return performance.now() - t0;
  }, index);
}

/** Digita a busca de uma vez e mede até a lista mostrar só o host procurado. */
async function searchTime(page: Page, query: string): Promise<number> {
  return page.evaluate(async (q) => {
    const frame = () => new Promise<number>((r) => requestAnimationFrame(r));
    const box = document.querySelector<HTMLInputElement>('input[aria-label="Buscar hosts"]');
    if (!box) throw new Error('caixa de busca não encontrada');
    // Setter nativo: o React só vê o evento se o valor mudar por baixo dele.
    const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
    const t0 = performance.now();
    descriptor?.set?.call(box, q);
    box.dispatchEvent(new Event('input', { bubbles: true }));
    for (let n = 0; n < 120; n++) {
      await frame();
      // Filtrado de verdade: a lista inteira tem só o host procurado.
      const items = [...document.querySelectorAll('[role=treeitem], [role=option]')];
      if (items.length === 1 && items[0]?.textContent.includes(q)) break;
    }
    return performance.now() - t0;
  }, query);
}

test('20 abas com saída contínua: troca de aba abaixo de 100 ms', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    for (let i = 0; i < TABS; i++) await page.keyboard.press('Control+Shift+T');
    await expect(page.getByRole('tab')).toHaveCount(TABS);

    // Uma aba despejando texto sem parar enquanto as outras são trocadas.
    await page.getByRole('tab').first().click();
    await page.locator('[role=tabpanel][data-session-id]').first().locator('.xterm').click();
    await page.keyboard.type('while ($true) { "y" }\r');
    await page.waitForTimeout(1_000);

    const times: number[] = [];
    for (let round = 0; round < 2; round++) {
      for (let i = 1; i < TABS; i++) times.push(await switchTime(page, i));
      times.push(await switchTime(page, 0));
    }
    times.sort((a, b) => a - b);
    const p95 = times[Math.floor(times.length * 0.95)] ?? Infinity;
    console.log(`[perf] ${test.info().title}: ${times.map((t) => t.toFixed(1)).join(' ')}`);
    test.info().annotations.push({
      type: 'troca de aba (ms)',
      description: `mediana ${times[times.length >> 1]?.toFixed(1)}, p95 ${p95.toFixed(1)}`,
    });
    expect(p95).toBeLessThan(SWITCH_MS);
  } finally {
    await app.close();
  }
});

test('busca em 1.000 hosts abaixo de 200 ms', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    await page.evaluate(async (count) => {
      const api = (
        window as unknown as {
          openterminal: { hosts: { create: (h: object) => Promise<{ ok: boolean }> } };
        }
      ).openterminal.hosts;
      for (let i = 0; i < count; i++) {
        const n = String(i).padStart(4, '0');
        const res = await api.create({
          label: `host-${n}`,
          address: `10.${String(i >> 8)}.${String(i & 255)}.1`,
          port: 22,
          username: `user${n}`,
          authMethod: 'ask',
          credentialId: null,
          groupId: null,
          legacyAlgorithms: false,
        });
        if (!res.ok) throw new Error(`falha ao criar host ${n}`);
      }
    }, HOSTS);
    // Bloquear e desbloquear recarrega a lista com os hosts criados pela ponte.
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await expect(unlockForm(page)).toBeVisible();
    await unlock(page);
    // A lista é virtualizada: confirma a carga pela busca, depois limpa.
    const search = page.getByLabel('Buscar hosts');
    await search.fill('host-0999');
    await expect(hostItem(page, 'host-0999')).toBeVisible({ timeout: 30_000 });
    await search.fill('');

    const times: number[] = [];
    for (const n of ['0777', '0042', '0500', '0999', '0001']) {
      times.push(await searchTime(page, `host-${n}`));
      await expect(hostItem(page, `host-${n}`)).toBeVisible();
    }
    times.sort((a, b) => a - b);
    const worst = times[times.length - 1] ?? Infinity;
    console.log(`[perf] ${test.info().title}: ${times.map((t) => t.toFixed(1)).join(' ')}`);
    test.info().annotations.push({
      type: 'busca (ms)',
      description: times.map((t) => t.toFixed(1)).join(', '),
    });
    expect(worst).toBeLessThan(SEARCH_MS);
  } finally {
    await app.close();
  }
});
