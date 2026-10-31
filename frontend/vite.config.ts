import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import react from '@vitejs/plugin-react';
import { defineConfig, type Plugin } from 'vite';

// Só no `wails dev`: o React Refresh injeta um script inline no HTML, que a CSP estrita barra.
// O build de produção mantém `script-src 'self'`.
const devCsp: Plugin = {
  name: 'openterminal-dev-csp',
  apply: 'serve',
  transformIndexHtml: (html) =>
    html.replace("script-src 'self'", "script-src 'self' 'unsafe-inline'"),
};

// A versão do produto vive no wails.json (a mesma do instalador e do executável).
const { info } = JSON.parse(
  readFileSync(resolve(import.meta.dirname, '..', 'wails.json'), 'utf8'),
) as { info: { productVersion: string } };
const version = info.productVersion;

export default defineConfig({
  plugins: [react(), devCsp],
  define: { __APP_VERSION__: JSON.stringify(version) },
  resolve: { alias: { '@shared': resolve(import.meta.dirname, 'src/shared') } },
  build: { outDir: 'dist', emptyOutDir: true },
});
