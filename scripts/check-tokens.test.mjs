import assert from 'node:assert/strict';
import test from 'node:test';

import { scanText } from './check-tokens.mjs';

test('reprova literais visuais em CSS', () => {
  const violations = scanText(
    'frontend/src/example.css',
    `.card {
  color: #fff;
  background: rgb(1, 2, 3);
  font-family: Consolas, monospace;
  box-shadow: 0 2px 8px rgba(0, 0, 0, .2);
}`,
  );
  assert.deepEqual(
    violations.map(({ line, reason }) => [line, reason]),
    [
      [2, 'cor literal; use um token do tema'],
      [3, 'cor literal; use um token do tema'],
      [5, 'cor literal; use um token do tema'],
      [4, 'font-family literal; use var(--token)'],
      [5, 'box-shadow literal; use var(--token)'],
    ],
  );
});

test('aceita tokens e ignora tema e arquivo de fontes gerado', () => {
  const clean = `.card {
  color: var(--text-primary);
  border-color: currentColor;
  background: transparent;
  font-family: var(--font-mono);
  box-shadow: var(--shadow-raised);
}`;
  assert.deepEqual(scanText('frontend/src/example.css', clean), []);
  assert.deepEqual(scanText('frontend/src/theme/tokens.css', '.x { color: #fff; }'), []);
  assert.deepEqual(scanText('frontend/src/styles/fonts.css', '@font-face { font-family: "Brand"; }'), []);
});

test('verifica style e permite SVG somente na biblioteca de UI', () => {
  const source = `<div style={{ color: '#fff', fontFamily: 'Consolas' }}><svg /></div>`;
  assert.deepEqual(
    scanText('frontend/src/features/Example.tsx', source).map(({ reason }) => reason),
    [
      'cor literal em style; use um token do tema',
      'fontFamily literal em style; use var(--token)',
      'SVG inline fora de frontend/src/ui',
    ],
  );
  assert.deepEqual(scanText('frontend/src/ui/Example.tsx', '<svg />'), []);
});
