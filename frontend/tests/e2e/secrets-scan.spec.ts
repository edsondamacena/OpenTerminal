// SC-108 (T098): depois de um fluxo completo, nenhum segredo legível no diretório de dados —
// inclusive SQLite (.db, -wal), pasta do WebView2 e despejos de falha.
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import { expect, test } from '@playwright/test';
import { KEYS_DIR, TEST_PASSPHRASE } from '../../../scripts/gen-test-keys.mjs';
import { SSHD } from '../../../scripts/sshd.mjs';
import { launchApp } from './helpers/launch';
import { waitWebViewGone } from './helpers/system';
import {
  acceptHostKey,
  addHost,
  createVault,
  expectTerminalLine,
  hostItem,
  MASTER_PASSWORD,
  sessionIdOf,
  typeInTerminal,
  unlock,
  waitConnected,
} from './helpers/ui';

const HOST_PASSWORD = 'pw-test-123';

function filesUnder(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...filesUnder(path));
    else if (entry.isFile()) out.push(path);
  }
  return out;
}

/** Trechos do corpo base64 da chave: início, meio e fim, longe dos cabeçalhos PEM. */
function keyFragments(pem: string): string[] {
  const body = pem
    .split('\n')
    .filter((line) => line && !line.startsWith('-----'))
    .join('');
  const at = [8, Math.floor(body.length / 2), body.length - 40];
  return at.map((i) => body.slice(i, i + 24));
}

function encodings(secret: string): { label: string; bytes: Buffer }[] {
  return [
    { label: 'utf8', bytes: Buffer.from(secret, 'utf8') },
    { label: 'utf16le', bytes: Buffer.from(secret, 'utf16le') },
    { label: 'base64', bytes: Buffer.from(Buffer.from(secret, 'utf8').toString('base64'), 'utf8') },
  ];
}

test('nenhum segredo legível no diretório de dados após o fluxo completo', async () => {
  test.setTimeout(120_000);
  const keyPem = readFileSync(join(KEYS_DIR, 'ed25519-passphrase'), 'utf8');
  let app = await launchApp();
  const { dataDir } = app;
  try {
    const { page } = app;
    await createVault(page);

    // Host com senha guardada no cofre, conectado.
    await addHost(page, {
      label: 'pw-host',
      address: SSHD.host,
      port: SSHD.port,
      username: 'pwuser',
      password: HOST_PASSWORD,
    });
    await hostItem(page, 'pw-host').dblclick();
    await acceptHostKey(page);
    await waitConnected(page, 'pw-host');
    const pw = await sessionIdOf(page, 'pw-host');
    await typeInTerminal(page, 'pw-host', 'echo pw-ok\r');
    await expectTerminalLine(page, pw, 'pw-ok');

    // Chave com passphrase importada colando o PEM, usada numa conexão.
    await page.getByRole('button', { name: 'Credenciais e chaves…' }).first().click();
    const creds = page.getByRole('dialog', { name: 'Credenciais e chaves' });
    await creds.getByLabel('Nome', { exact: true }).fill('chave-com-senha');
    await creds.getByLabel('Ou cole a chave privada').fill(keyPem);
    await creds.getByLabel(/Senha da chave/).fill(TEST_PASSPHRASE);
    await creds.getByRole('button', { name: 'Salvar' }).click();
    await expect(creds.getByRole('listitem').filter({ hasText: 'chave-com-senha' })).toBeVisible();
    await creds.getByRole('button', { name: 'Fechar' }).last().click();
    await addHost(page, {
      label: 'key-host',
      address: SSHD.host,
      port: SSHD.port,
      username: 'keyuser',
      method: 'Chave privada',
      credential: 'chave-com-senha',
    });
    await hostItem(page, 'key-host').dblclick();
    await waitConnected(page, 'key-host');
    const key = await sessionIdOf(page, 'key-host');
    await typeInTerminal(page, 'key-host', 'whoami\r');
    await expectTerminalLine(page, key, 'keyuser');

    // Bloquear, fechar e reabrir: o que ficou em disco é o que conta.
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await app.close();
    app = await launchApp({ dataDir });
    await unlock(app.page);
    await app.close();
    await waitWebViewGone(dataDir);

    const files = filesUnder(dataDir);
    expect(files.length).toBeGreaterThan(0);
    expect(files.some((f) => /\.db$/.test(f))).toBe(true);

    const secrets: Record<string, string> = {
      'senha mestra': MASTER_PASSWORD,
      'senha do host': HOST_PASSWORD,
      passphrase: TEST_PASSPHRASE,
    };
    keyFragments(keyPem).forEach((fragment, i) => {
      secrets[`trecho ${String(i + 1)} da chave`] = fragment;
    });

    const found: string[] = [];
    for (const file of files) {
      const data = readFileSync(file);
      for (const [name, secret] of Object.entries(secrets)) {
        for (const { label, bytes } of encodings(secret)) {
          if (data.includes(bytes)) found.push(`${relative(dataDir, file)}: ${name} (${label})`);
        }
      }
    }
    expect(found, 'segredos legíveis encontrados').toEqual([]);

    // Sem despejos de falha nem logs do app. Os .log do WebView2 são diários do LevelDB e
    // diagnósticos do Chromium; o conteúdo deles já passou pela varredura acima.
    const webview = /^webview[\\/]/i;
    const dumps = files.filter((f) => {
      const rel = relative(dataDir, f);
      return (
        /\.dmp$/i.test(rel) ||
        /Crashpad[\\/]reports[\\/]/i.test(rel) ||
        (/\.log$/i.test(rel) && !webview.test(rel))
      );
    });
    expect(dumps.map((f) => `${relative(dataDir, f)} (${String(statSync(f).size)} B)`)).toEqual(
      [],
    );
  } finally {
    await app.close();
  }
});
