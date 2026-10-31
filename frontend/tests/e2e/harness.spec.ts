// Fumaça do harness: o build e2e abre, o Playwright conecta pelo CDP e o TestService responde.
import { expect, test } from '@playwright/test';
import { launchApp, testService } from './helpers/launch';

test('build e2e abre e expõe o TestService', async () => {
  const app = await launchApp();
  try {
    await expect(app.page.locator('#root')).toBeAttached();
    expect(await testService<number>(app.page, 'Goroutines')).toBeGreaterThan(0);
    await testService(app.page, 'SetSize', 1000, 700);
    await expect
      .poll(() => app.page.evaluate(() => window.innerWidth), { timeout: 5000 })
      .toBeLessThanOrEqual(1000);
  } finally {
    await app.close();
  }
});
