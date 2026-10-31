// US3 — grupos, mover, buscar e duplicar (T041; da 001 T071).
import { expect, test, type Page } from '@playwright/test';
import { launchApp } from './helpers/launch';
import { addHost, chooseOption, createVault, groupItem, hostItem, unlock } from './helpers/ui';

async function nameGroup(page: Page, title: string, name: string) {
  const dialog = page.getByRole('dialog', { name: title });
  await dialog.getByLabel('Nome do grupo').fill(name);
  await dialog.getByRole('button', { name: 'Salvar' }).click();
  await expect(dialog).toBeHidden();
}

async function menu(page: Page, row: ReturnType<typeof hostItem>, item: string) {
  await row.click({ button: 'right' });
  await page.getByRole('menuitem', { name: item }).click();
}

test('organizar hosts em grupos, buscar e duplicar', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    const ask = { port: 22, username: 'deploy', method: 'Perguntar ao conectar' } as const;
    await addHost(page, { ...ask, label: 'acme-web', address: '10.1.2.3' });
    await addHost(page, { ...ask, label: 'db-01', address: '10.9.8.7' });

    // Grupo e subgrupo.
    await page.getByRole('button', { name: 'Novo grupo' }).click();
    await nameGroup(page, 'Novo grupo', 'Clientes');
    await menu(page, groupItem(page, 'Clientes'), 'Novo subgrupo');
    await nameGroup(page, 'Novo subgrupo', 'ACME');
    await expect(groupItem(page, 'ACME')).toHaveAttribute('aria-level', '2');

    // Mover por arrastar e por menu.
    await hostItem(page, 'acme-web').dragTo(groupItem(page, 'ACME'));
    await expect(hostItem(page, 'acme-web')).toHaveAttribute('aria-level', '3');
    await menu(page, hostItem(page, 'db-01'), 'Mover para…');
    const move = page.getByRole('dialog', { name: 'Mover para grupo' });
    await chooseOption(move, 'Destino', 'Clientes');
    await move.getByRole('button', { name: 'Confirmar' }).click();
    await expect(hostItem(page, 'db-01')).toHaveAttribute('aria-level', '2');

    // Recolher esconde os descendentes.
    await groupItem(page, 'Clientes').click();
    await expect(hostItem(page, 'db-01')).toBeHidden();
    await groupItem(page, 'Clientes').click();
    await expect(hostItem(page, 'db-01')).toBeVisible();

    // Busca filtra enquanto digita, com o caminho do grupo.
    const search = page.getByRole('searchbox', { name: 'Buscar hosts' });
    await search.pressSequentially('ACME');
    await expect(page.getByRole('option')).toHaveCount(1);
    await expect(hostItem(page, 'acme-web')).toContainText('Clientes / ACME');
    await search.fill('');
    await search.pressSequentially('10.9.8');
    await expect(page.getByRole('option')).toHaveCount(1);
    await expect(hostItem(page, 'db-01')).toBeVisible();
    await search.fill('nada-casa');
    await expect(page.getByText('Nenhum host encontrado.')).toBeVisible();
    await search.press('Escape');
    await expect(page.getByRole('tree')).toBeVisible();

    // Duplicar.
    await menu(page, hostItem(page, 'db-01'), 'Duplicar');
    await expect(hostItem(page, 'db-01 (cópia)')).toHaveAttribute('aria-level', '2');

    // Excluir grupo com confirmação: hosts e subgrupos sobem para a raiz.
    await menu(page, groupItem(page, 'Clientes'), 'Excluir');
    const confirm = page.getByRole('dialog', { name: 'Excluir grupo' });
    await expect(confirm).toContainText('nenhum host é excluído');
    await confirm.getByRole('button', { name: 'Excluir' }).click();
    await expect(groupItem(page, 'Clientes')).toHaveCount(0);
    await expect(groupItem(page, 'ACME')).toHaveAttribute('aria-level', '1');
    await expect(hostItem(page, 'db-01')).toHaveAttribute('aria-level', '1');
    await expect(hostItem(page, 'db-01 (cópia)')).toHaveAttribute('aria-level', '1');
    await expect(hostItem(page, 'acme-web')).toHaveAttribute('aria-level', '2');
  } finally {
    await app.close();
  }
});

test('1.000 hosts: lista virtualizada e busca imediata (SC-003)', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    // Semeia pela própria ponte: 5 grupos com 4 subgrupos e 1.000 hosts, 10% na raiz.
    await page.evaluate(async (count) => {
      type Res<T> = { ok: true; value: T } | { ok: false; error: { code: string } };
      const api = (
        window as unknown as {
          openterminal: {
            groups: {
              create: (name: string, parentId: string | null) => Promise<Res<{ id: string }>>;
            };
            hosts: { create: (input: Record<string, unknown>) => Promise<Res<unknown>> };
          };
        }
      ).openterminal;
      const leaves: string[] = [];
      for (const name of ['Produção', 'Homologação', 'Clientes', 'Laboratório', 'Legado']) {
        const top = await api.groups.create(name, null);
        if (!top.ok) throw new Error(top.error.code);
        for (const sub of ['Web', 'Banco', 'Filas', 'Monitoramento']) {
          const child = await api.groups.create(sub, top.value.id);
          if (!child.ok) throw new Error(child.error.code);
          leaves.push(child.value.id);
        }
        leaves.push(top.value.id);
      }
      for (let i = 0; i < count; i++) {
        const res = await api.hosts.create({
          label: `srv-${String(i + 1).padStart(5, '0')}`,
          address: `10.0.${String((i >> 8) & 255)}.${String(i & 255)}`,
          port: 22,
          username: 'deploy',
          authMethod: 'ask',
          credentialId: null,
          groupId: i % 10 === 0 ? null : (leaves[i % leaves.length] ?? null),
        });
        if (!res.ok) throw new Error(res.error.code);
      }
    }, 1000);
    // Reabrir a lista a partir do cofre, como numa execução nova.
    await page.getByRole('button', { name: 'Bloquear cofre' }).click();
    await unlock(page);
    await expect(groupItem(page, 'Produção')).toBeVisible();
    // Só a janela visível é renderizada.
    const rendered = await page.getByRole('treeitem').count();
    expect(rendered).toBeGreaterThan(10);
    expect(rendered).toBeLessThan(200);

    const search = page.getByRole('searchbox', { name: 'Buscar hosts' });
    await search.fill('srv-00999');
    await expect(page.getByRole('option')).toHaveCount(1);
    await expect(hostItem(page, 'srv-00999')).toBeVisible();
  } finally {
    await app.close();
  }
});

test('grupo recolhido continua recolhido depois de reiniciar (FR-167)', async () => {
  let app = await launchApp();
  const { dataDir } = app;
  try {
    await createVault(app.page);
    await app.page.getByRole('button', { name: 'Novo grupo' }).click();
    await nameGroup(app.page, 'Novo grupo', 'Clientes');
    await addHost(app.page, {
      label: 'acme-web',
      address: '10.1.2.3',
      port: 22,
      username: 'deploy',
      method: 'Perguntar ao conectar',
    });
    await hostItem(app.page, 'acme-web').dragTo(groupItem(app.page, 'Clientes'));
    await groupItem(app.page, 'Clientes').click();
    await expect(groupItem(app.page, 'Clientes')).toHaveAttribute('aria-expanded', 'false');
    await expect(hostItem(app.page, 'acme-web')).toBeHidden();
    // A preferência é salva depois da interface mudar: espera chegar ao disco.
    await expect
      .poll(async () => {
        return app.page.evaluate(async () => {
          type Res = { ok: true; value: { collapsedGroups: string[] } } | { ok: false };
          const api = (
            window as unknown as { openterminal: { app: { getUiSettings: () => Promise<Res> } } }
          ).openterminal;
          const res = await api.app.getUiSettings();
          return res.ok ? res.value.collapsedGroups.length : -1;
        });
      })
      .toBe(1);
    await app.close();

    app = await launchApp({ dataDir });
    await unlock(app.page);
    await expect(groupItem(app.page, 'Clientes')).toHaveAttribute('aria-expanded', 'false');
    await expect(hostItem(app.page, 'acme-web')).toBeHidden();
    await groupItem(app.page, 'Clientes').click();
    await expect(hostItem(app.page, 'acme-web')).toBeVisible();
  } finally {
    await app.close();
  }
});
