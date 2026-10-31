// App offline: a interface não faz requisições de rede (T041; da 001 T102, FR-027).
import { expect, test } from '@playwright/test';
import { launchApp } from './helpers/launch';
import { createVault, openLocalTab, waitPowerShellPrompt } from './helpers/ui';

test('requisições externas são barradas e o fluxo básico não usa a rede', async () => {
  const app = await launchApp();
  const { page } = app;
  // Assets embutidos vêm da origem do próprio app. Fixa: `page.url()` pode ainda ser
  // `about:blank` se o primeiro carregamento não terminou.
  const origin = 'http://wails.localhost';
  const external: string[] = [];
  page.on('request', (req) => {
    const url = req.url();
    if (!url.startsWith(origin) && !/^(data|blob|devtools):/.test(url)) external.push(url);
  });
  try {
    await createVault(page);
    const id = await openLocalTab(page, 'PowerShell');
    await waitPowerShellPrompt(page, id);

    // CSP `connect-src 'self'`: o fetch para fora falha.
    const fromRenderer = await page.evaluate(async () => {
      try {
        await fetch('https://example.com');
        return 'ok';
      } catch (e) {
        return String(e);
      }
    });
    expect(fromRenderer).not.toBe('ok');

    // Só a tentativa explícita acima pode aparecer; nada do fluxo em si.
    expect(external.filter((url) => !url.startsWith('https://example.com'))).toEqual([]);
  } finally {
    await app.close();
  }
});
