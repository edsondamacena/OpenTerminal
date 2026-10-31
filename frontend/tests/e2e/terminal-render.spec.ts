// US4 — renderização do terminal: cores, largura Unicode, copiar/colar e histórico (T041; da 001 T079).
import { expect, test } from '@playwright/test';
import { launchApp } from './helpers/launch';
import { readClipboard } from './helpers/system';
import {
  createVault,
  expectTerminalLine,
  openLocalTab,
  selectInTerminal,
  terminalCells,
  terminalText,
  typeInTerminal,
  waitPowerShellPrompt,
} from './helpers/ui';

const PS = 'PowerShell';

test('cores, Unicode, copiar/colar e 10.000 linhas de histórico', async () => {
  const app = await launchApp();
  const { page } = app;
  try {
    await createVault(page);
    const id = await openLocalTab(page, PS);
    await waitPowerShellPrompt(page, id);

    // 256 cores e true color.
    await typeInTerminal(
      page,
      PS,
      'Write-Host ("A" + [char]27 + "[38;5;196mR" + [char]27 + "[0m B" + [char]27 + "[38;2;10;200;30mT" + [char]27 + "[0m:cores")\r',
    );
    await expectTerminalLine(page, id, 'AR BT:cores');
    const colors = await terminalCells(page, id, 'AR BT:cores');
    expect(colors.find((c) => c.chars === 'A')?.fg).toBe('default');
    expect(colors.find((c) => c.chars === 'R')).toMatchObject({ fg: 'palette', fgColor: 196 });
    expect(colors.find((c) => c.chars === 'T')).toMatchObject({
      fg: 'rgb',
      fgColor: (10 << 16) | (200 << 8) | 30,
    });

    // CJK e emoji ocupam duas colunas (unicode11).
    await typeInTerminal(
      page,
      PS,
      'Write-Host ("W:" + [char]0x6F22 + [char]0x5B57 + [char]::ConvertFromUtf32(0x1F600) + "|fim")\r',
    );
    await expect.poll(async () => (await terminalText(page, id)).includes('|fim')).toBe(true);
    const wide = await terminalCells(page, id, '|fim');
    expect(wide.find((c) => c.chars === '漢')?.width).toBe(2);
    expect(wide.find((c) => c.chars === '字')?.width).toBe(2);
    expect(wide.find((c) => c.chars === '😀')?.width).toBe(2);

    // Copiar a seleção (Ctrl+Shift+C) e colar (Ctrl+Shift+V).
    await typeInTerminal(page, PS, 'Write-Host COPIA-123\r');
    await expectTerminalLine(page, id, 'COPIA-123');
    await selectInTerminal(page, id, 'COPIA-123');
    await page.keyboard.press('Control+Shift+C');
    await expect.poll(() => readClipboard()).toBe('COPIA-123');
    await page.keyboard.press('Control+Shift+V');
    await expect
      .poll(async () =>
        (await terminalText(page, id)).split('\n').some((l) => /PS .*> ?COPIA-123/.test(l)),
      )
      .toBe(true);
    await page.keyboard.press('Escape');

    // 15.000 linhas: o histórico guarda pelo menos 10.000 (FR-015).
    await typeInTerminal(page, PS, '1..15000 | ForEach-Object { "linha $_" }\r');
    await expectTerminalLine(page, id, 'linha 15000');
    const lines = (await terminalText(page, id)).split('\n').map((l) => l.trimEnd());
    expect(lines.length).toBeGreaterThanOrEqual(10_000);
    expect(lines).toContain('linha 6000');
    expect(lines).not.toContain('linha 1');
  } finally {
    await app.close();
  }
});
