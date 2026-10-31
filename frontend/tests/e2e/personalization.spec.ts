// US4 — personalização do terminal aplicada ao vivo, importar/exportar esquemas e perfil por host (T076).
import { existsSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { SSHD } from '../../../scripts/sshd.mjs';
import { launchApp, testService } from './helpers/launch';
import { REPO_ROOT } from './helpers/paths';
import {
  acceptHostKey,
  addHost,
  createVault,
  hostItem,
  openLocalTab,
  sessionIdOf,
  terminalLook,
  waitConnected,
} from './helpers/ui';

const FIXTURES = resolve(REPO_ROOT, 'tests/fixtures/schemes');
/** Mudança de perfil precisa chegar a todas as abas abertas em menos de 1 s. */
const LIVE_MS = 1_000;

async function dialogReturns(page: Page, path: string) {
  await testService(page, 'SetDialogPath', path);
}

async function expectLook(
  page: Page,
  sessionId: string,
  want: Partial<{ fontFamily: string; fontSize: number; background: string }>,
) {
  await expect
    .poll(
      async () => {
        const look = await terminalLook(page, sessionId);
        if (!look) return null;
        return {
          ...(want.fontFamily !== undefined && {
            fontFamily: look.fontFamily.includes(want.fontFamily),
          }),
          ...(want.fontSize !== undefined && { fontSize: look.fontSize }),
          ...(want.background !== undefined && { background: look.background.toLowerCase() }),
        };
      },
      { timeout: LIVE_MS, intervals: [50] },
    )
    .toEqual({
      ...(want.fontFamily !== undefined && { fontFamily: true }),
      ...(want.fontSize !== undefined && { fontSize: want.fontSize }),
      ...(want.background !== undefined && { background: want.background }),
    });
}

test('fonte, tamanho e esquema ao vivo; importar, exportar; perfil próprio do host', async () => {
  const app = await launchApp();
  const { page } = app;
  const out = mkdtempSync(join(tmpdir(), 'openterminal-e2e-schemes-'));
  try {
    await createVault(page);
    await addHost(page, {
      label: 'pw-host',
      address: SSHD.host,
      port: SSHD.port,
      username: 'pwuser',
      password: 'pw-test-123',
    });

    // Três abas: uma SSH e duas locais.
    await hostItem(page, 'pw-host').dblclick();
    await acceptHostKey(page);
    await waitConnected(page, 'pw-host');
    const ssh = await sessionIdOf(page, 'pw-host');
    const ps = await openLocalTab(page, 'PowerShell');
    const cmd = await openLocalTab(page, 'Prompt de Comando');
    const all = [ssh, ps, cmd];

    await page.getByRole('button', { name: 'Configurações' }).first().click();
    const settings = page.getByRole('region', { name: 'Configurações' });
    await settings.getByRole('button', { name: 'Terminal', exact: true }).click();

    // Fonte e tamanho: aplicados em todas as abas, sem reconectar.
    await settings.getByRole('radio', { name: 'Cascadia Code' }).click();
    for (const id of all) await expectLook(page, id, { fontFamily: 'Cascadia Code' });
    await settings.getByLabel('Tamanho').fill('18');
    for (const id of all) await expectLook(page, id, { fontSize: 18 });

    // Importar do Windows Terminal: o esquema importado vira o atual.
    await dialogReturns(page, join(FIXTURES, 'windows-terminal.json'));
    await settings.getByRole('button', { name: 'Importar…' }).click();
    await expect(page.getByText('1 esquema importado.')).toBeVisible();
    await expect(settings.getByRole('radio', { name: 'Fixture WT' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    for (const id of all) await expectLook(page, id, { background: '#101010' });

    // Importar do iTerm2 (.itermcolors).
    await dialogReturns(page, join(FIXTURES, 'iterm-srgb.itermcolors'));
    await settings.getByRole('button', { name: 'Importar…' }).click();
    await expect(page.getByText(/esquemas? importados?\./).last()).toBeVisible();
    for (const id of all) await expectLook(page, id, { background: '#000000' });

    // Exportar o esquema atual e importar de volta.
    await settings.getByRole('radio', { name: 'Fixture WT' }).click();
    const exported = join(out, 'exportado.json');
    await dialogReturns(page, exported);
    await settings.getByRole('button', { name: 'Exportar…' }).click();
    await expect.poll(() => existsSync(exported)).toBe(true);
    const before = await settings.getByRole('radio').count();
    await dialogReturns(page, exported);
    await settings.getByRole('button', { name: 'Importar…' }).click();
    // Nome repetido ganha sufixo em vez de falhar (cartão SCHNAME); soft para os passos seguintes rodarem.
    await expect.soft(settings.getByRole('radio')).toHaveCount(before + 1);
    for (const id of all) await expectLook(page, id, { background: '#101010' });

    // As abas seguem as mesmas sessões (nada reconectou).
    await page.keyboard.press('Escape');
    await expect(settings).toBeHidden();
    expect(await sessionIdOf(page, 'pw-host')).toBe(ssh);
    expect(await sessionIdOf(page, 'PowerShell')).toBe(ps);
    expect(await sessionIdOf(page, 'Prompt de Comando')).toBe(cmd);

    // Perfil próprio do host: muda só a aba desse host.
    await hostItem(page, 'pw-host').click({ button: 'right' });
    await page.getByRole('menuitem', { name: 'Editar' }).click();
    const form = page.getByRole('dialog', { name: 'Editar host' });
    await form.getByRole('button', { name: 'Perfil de terminal próprio…' }).click();
    const own = page.getByRole('dialog', { name: 'Terminal de pw-host' });
    await own.getByLabel('Tamanho').fill('22');
    await own.getByRole('button', { name: 'Salvar' }).click();
    await expect(own).toBeHidden();
    await expectLook(page, ssh, { fontSize: 22 });
    await expectLook(page, ps, { fontSize: 18 });
    await expectLook(page, cmd, { fontSize: 18 });
    expect(await sessionIdOf(page, 'pw-host')).toBe(ssh);
  } finally {
    await app.close();
  }
});
