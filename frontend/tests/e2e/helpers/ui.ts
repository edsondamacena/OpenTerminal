import { expect, type Locator, type Page } from '@playwright/test';

export const MASTER_PASSWORD = 'senha-teste-1';

export const sidebarHosts = (page: Page) => page.getByRole('heading', { name: 'Hosts' });
export const unlockForm = (page: Page) => page.getByRole('form', { name: 'Cofre bloqueado' });

export async function createVault(page: Page, password = MASTER_PASSWORD): Promise<void> {
  await page.getByLabel('Senha mestra', { exact: true }).fill(password);
  await page.getByLabel('Confirme a senha mestra', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Criar cofre' }).click();
  await expect(sidebarHosts(page)).toBeVisible({ timeout: 30_000 });
}

export async function unlock(page: Page, password = MASTER_PASSWORD): Promise<void> {
  const form = unlockForm(page);
  await form.getByLabel('Senha mestra', { exact: true }).fill(password);
  await form.getByRole('button', { name: 'Desbloquear' }).click();
}

export interface HostFields {
  label: string;
  address: string;
  port: number;
  username: string;
  /** Senha nova criada no próprio formulário. */
  password?: string;
  /** Nome de credencial já salva. */
  credential?: string;
  method?: 'Senha' | 'Chave privada' | 'Perguntar ao conectar' | 'Agente SSH do Windows';
  legacy?: boolean;
}

/** Escolhe uma opção do Select próprio (a lista abre num portal, fora do diálogo). */
export async function chooseOption(scope: Locator, label: string, option: string): Promise<void> {
  await scope.getByRole('combobox', { name: label, exact: true }).click();
  await scope
    .page()
    .getByRole('listbox', { name: label, exact: true })
    .getByRole('option', { name: option, exact: true })
    .click();
  await expect(scope.getByRole('combobox', { name: label, exact: true })).toHaveText(option);
}

export async function addHost(page: Page, host: HostFields): Promise<void> {
  await page.getByRole('button', { name: 'Novo host' }).click();
  const dialog = page.getByRole('dialog', { name: 'Novo host' });
  await dialog.getByLabel('Nome', { exact: true }).fill(host.label);
  await dialog.getByLabel('Endereço', { exact: true }).fill(host.address);
  await dialog.getByLabel('Porta', { exact: true }).fill(String(host.port));
  await dialog.getByLabel('Usuário', { exact: true }).fill(host.username);
  if (host.method) await chooseOption(dialog, 'Autenticação', host.method);
  if (host.credential) {
    await chooseOption(dialog, 'Credencial', host.credential);
  } else if (host.password !== undefined) {
    await dialog.getByLabel('Senha', { exact: true }).fill(host.password);
  }
  if (host.legacy) await dialog.getByLabel('Algoritmos legados (inseguros)').check();
  await dialog.getByRole('button', { name: 'Salvar' }).click();
  await expect(dialog).toBeHidden();
}

export async function importKey(page: Page, name: string, pem: string): Promise<void> {
  await page.getByRole('button', { name: 'Credenciais e chaves…' }).first().click();
  const dialog = page.getByRole('dialog', { name: 'Credenciais e chaves' });
  await dialog.getByLabel('Nome', { exact: true }).fill(name);
  await dialog.getByLabel('Ou cole a chave privada').fill(pem);
  await dialog.getByRole('button', { name: 'Salvar' }).click();
  await expect(dialog.getByRole('listitem').filter({ hasText: name })).toBeVisible();
  await dialog.getByRole('button', { name: 'Fechar' }).last().click();
  await expect(dialog).toBeHidden();
}

/** Linha de host (na árvore ou nos resultados da busca) com rótulo exato. */
export const hostItem = (page: Page, label: string) =>
  page
    .locator('[role=treeitem], [role=option]')
    .filter({ has: page.getByText(label, { exact: true }) });

export const groupItem = (page: Page, name: string) =>
  page
    .locator('[role=treeitem][aria-expanded]')
    .filter({ has: page.getByText(name, { exact: true }) });

export const tab = (page: Page, title: string) => page.getByRole('tab', { name: title });

/** Painel do terminal da aba (aria-label = título). */
export const terminalPanel = (page: Page, title: string): Locator =>
  page.getByRole('tabpanel', { name: title, includeHidden: true });

export async function sessionIdOf(page: Page, title: string): Promise<string> {
  const id = await terminalPanel(page, title).getAttribute('data-session-id');
  if (!id) throw new Error(`aba sem sessão: ${title}`);
  return id;
}

export interface ProbeCell {
  chars: string;
  width: number;
  fg: 'default' | 'palette' | 'rgb';
  fgColor: number;
}

export interface ProbeLook {
  fontFamily: string;
  fontSize: number;
  background: string;
  foreground: string;
}

/** Probe exposto pelo renderer em E2E (frontend/src/features/terminal/testProbe.ts). */
interface Probe {
  text: (id: string) => string | null;
  cells: (id: string, needle: string) => ProbeCell[] | null;
  select: (id: string, needle: string) => boolean;
  size: (id: string) => { cols: number; rows: number } | null;
  look: (id: string) => ProbeLook | null;
  resizes: (id: string) => number;
}

declare global {
  interface Window {
    __openterminalProbe: Probe;
  }
}

export async function terminalText(page: Page, sessionId: string): Promise<string> {
  return page.evaluate((id) => window.__openterminalProbe.text(id) ?? '', sessionId);
}

export async function terminalCells(
  page: Page,
  sessionId: string,
  needle: string,
): Promise<ProbeCell[]> {
  return page.evaluate(([id, n]) => window.__openterminalProbe.cells(id, n) ?? [], [
    sessionId,
    needle,
  ] as const);
}

export async function terminalSize(page: Page, sessionId: string) {
  return page.evaluate((id) => window.__openterminalProbe.size(id), sessionId);
}

/** Fonte, tamanho e cores aplicados ao xterm da aba. */
export async function terminalLook(page: Page, sessionId: string): Promise<ProbeLook | null> {
  return page.evaluate((id) => window.__openterminalProbe.look(id), sessionId);
}

export async function resizeCount(page: Page, sessionId: string): Promise<number> {
  return page.evaluate((id) => window.__openterminalProbe.resizes(id), sessionId);
}

export async function selectInTerminal(page: Page, sessionId: string, needle: string) {
  const ok = await page.evaluate(([id, n]) => window.__openterminalProbe.select(id, n), [
    sessionId,
    needle,
  ] as const);
  if (!ok) throw new Error(`texto não encontrado no terminal: ${needle}`);
}

/** Espera uma linha do terminal igual a `line` (ignora espaços à direita). */
export async function expectTerminalLine(page: Page, sessionId: string, line: string) {
  await expect
    .poll(async () => (await terminalText(page, sessionId)).split('\n').map((l) => l.trimEnd()), {
      timeout: 20_000,
    })
    .toContain(line);
}

export async function typeInTerminal(page: Page, title: string, text: string): Promise<void> {
  await terminalPanel(page, title).locator('.xterm').click();
  await page.keyboard.type(text);
}

export async function waitConnected(page: Page, title: string): Promise<void> {
  await expect(tab(page, title).locator('[data-state]')).toHaveAttribute(
    'data-state',
    'connected',
    {
      timeout: 30_000,
    },
  );
}

export async function acceptHostKey(page: Page): Promise<void> {
  const dialog = page.getByRole('dialog', { name: 'Confirme a chave do servidor' });
  await expect(dialog.getByTestId('fingerprint')).toContainText('SHA256:');
  await dialog.getByRole('button', { name: 'Aceitar e conectar' }).click();
}

/** Abre uma aba local pelo menu "+" e devolve o id da sessão. */
export async function openLocalTab(page: Page, shell: 'PowerShell' | 'Prompt de Comando') {
  await page.getByRole('button', { name: 'Novo terminal local' }).click();
  await page.getByRole('menuitem', { name: shell, exact: true }).click();
  await expect(tab(page, shell)).toBeVisible();
  return sessionIdOf(page, shell);
}

/** Espera o prompt do PowerShell aparecer (shell pronto para digitar). */
export async function waitPowerShellPrompt(page: Page, sessionId: string) {
  await expect
    .poll(async () => /PS [A-Z]:\\.*> ?$/m.test(await terminalText(page, sessionId)), {
      timeout: 30_000,
    })
    .toBe(true);
}
