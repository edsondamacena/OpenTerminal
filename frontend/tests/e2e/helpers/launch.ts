// Abre o build de testes (tag e2e) e conecta o Playwright ao WebView2 pelo DevTools Protocol
// (research R18). Cada app recebe diretório de dados e porta CDP próprios.
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { chromium, type Browser, type Page } from '@playwright/test';
import { E2E_EXE } from './paths';

export interface LaunchedApp {
  page: Page;
  dataDir: string;
  process: ChildProcess;
  browser: Browser;
  /** Encerra o processo sem passar pela confirmação de fechar (FR-028). */
  close: () => Promise<void>;
}

let nextPort = 9400 + Math.floor(Math.random() * 400);

async function waitForCdp(port: number, child: ChildProcess, timeoutMs = 30_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(`app saiu com código ${String(child.exitCode)}`);
    try {
      const res = await fetch(`http://127.0.0.1:${String(port)}/json/version`);
      if (res.ok) return;
    } catch {
      // WebView2 ainda subindo.
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`WebView2 não abriu a porta CDP ${String(port)}`);
}

async function appPage(browser: Browser): Promise<Page> {
  for (let i = 0; i < 100; i++) {
    for (const context of browser.contexts()) {
      const page = context.pages().find((p) => !p.url().startsWith('devtools:'));
      if (page) return page;
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error('página do app não encontrada');
}

export async function launchApp(
  options: {
    dataDir?: string;
    env?: Record<string, string>;
    /** Animações reais. Por padrão a suíte emula "reduzir movimento": as cópias de saída de
     * diálogos e menus (leaveGhost) não existem e os seletores não as encontram. */
    motion?: boolean;
  } = {},
): Promise<LaunchedApp> {
  const dataDir = options.dataDir ?? mkdtempSync(join(tmpdir(), 'openterminal-e2e-'));
  const port = nextPort++;
  const child = spawn(E2E_EXE, [], {
    env: {
      ...process.env,
      OPENTERMINAL_DATA_DIR: dataDir,
      OPENTERMINAL_E2E_CDP_PORT: String(port),
      ...options.env,
    },
    stdio: 'ignore',
  });
  await waitForCdp(port, child);
  const browser = await chromium.connectOverCDP(`http://127.0.0.1:${String(port)}`);
  const page = await appPage(browser);
  await page.waitForLoadState('domcontentloaded');
  if (!options.motion) await page.emulateMedia({ reducedMotion: 'reduce' });
  const close = async (): Promise<void> => {
    await browser.close().catch(() => undefined);
    // Morto por sinal, exitCode fica null e signalCode preenchido: fechar de novo não espera.
    if (child.exitCode === null && child.signalCode === null) {
      const exited = new Promise((r) => child.once('exit', r));
      child.kill();
      await exited;
    }
  };
  return { page, dataDir, process: child, browser, close };
}

/** Chama um método do TestService (só existe no build e2e). */
export async function testService<T>(page: Page, method: string, ...args: unknown[]): Promise<T> {
  return page.evaluate(
    async ([m, a]) => {
      const svc = (
        window as unknown as {
          go: { testsvc: { TestService: Record<string, (...x: unknown[]) => Promise<unknown>> } };
        }
      ).go.testsvc.TestService;
      const fn = svc[m];
      if (!fn) throw new Error(`TestService.${m} não existe`);
      return fn(...a);
    },
    [method, args] as const,
  ) as Promise<T>;
}
