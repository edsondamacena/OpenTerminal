---

description: "Task list for MVP do cliente SSH ShellVault"
---

# Tasks: MVP do cliente SSH ShellVault

**Input**: Design documents from `specs/001-mvp-ssh-client/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ipc-bridge.md, quickstart.md

**Tests**: obrigatórios pela constituição (Princípio IV — Testes Primeiro). Testes de cada
história MUST ser escritos antes da implementação e falhar antes de passar. Integração
SSH/SFTP/forwarding roda contra o OpenSSH em contêiner (`tests/fixtures/sshd`).

**Organization**: tarefas agrupadas por história de usuário para implementação e teste
independentes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: história de usuário (US1…US6)
- Caminhos relativos à raiz do repositório; layout em plan.md ("Project Structure")

## Convenções que valem para todas as tarefas

- TypeScript `strict` + `noUncheckedIndexedAccess`; identificadores em inglês; textos de
  interface só via `src/renderer/i18n/pt-BR.ts`.
- Toda função da ponte segue contracts/ipc-bridge.md ("Regras gerais"): retorna
  `Result<T>`, valida entrada com zod, verifica `senderFrame`, funções 🔒 respondem
  `VAULT_LOCKED` com cofre bloqueado, nunca devolve segredos.
- Nenhum `console.*` fora de `import.meta.env.DEV`; nenhum log em arquivo (FR-030).
- Ícones e componentes visuais são próprios (FR-031, Princípio I).
- Uma tarefa só conclui com `npm run lint`, `npm run typecheck` e testes passando.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: inicializar o projeto Electron + ferramentas

- [X] T001 Criar `package.json` (nome `shellvault`, `"type": "module"`, `private: true`, licença MIT) e instalar dependências fixando versões de research.md: runtime `electron@45`, `react@19`, `react-dom@19`, `zustand@5`, `@xterm/xterm@6`, `@xterm/addon-fit`, `@xterm/addon-webgl`, `@xterm/addon-unicode11`, `ssh2@1.17`, `node-pty@1.1`, `libsodium-wrappers-sumo@0.8`, `zod@4`; dev `electron-vite@6`, `electron-builder@26`, `typescript@~6.0`, `@types/node`, `@types/react`, `@types/react-dom`, `@types/ssh2`, `vitest@5`, `@playwright/test@1.64`, `eslint@9`, `typescript-eslint`, `eslint-plugin-react-hooks`, `prettier`; adicionar `.gitignore` (node_modules, out, dist, coverage, test-results, tests/fixtures/keys/generated)
- [X] T002 Criar `electron.vite.config.ts` com entradas `src/main/index.ts`, `src/preload/index.ts`, `src/renderer/index.html`; alias `@shared` → `src/shared`; `ssh2`, `node-pty` e `libsodium-wrappers-sumo` como externos no main
- [X] T003 [P] Criar `tsconfig.json` (references), `tsconfig.node.json` (main, preload, shared; lib ES2023, types node) `tsconfig.web.json` (renderer, shared; lib DOM) e `tsconfig.test.json` (`tests/**` + `scripts/**`, libs DOM e node, `noEmit`, sem `composite`, para testes que importam código de main e de renderer), todos com `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`
- [X] T004 [P] Configurar `eslint.config.js` (typescript-eslint strict-type-checked, react-hooks, regra `no-console` erro, `no-restricted-imports` impedindo `electron`/`node:*`/`ssh2`/`node-pty` em `src/renderer/**` e `src/shared/**`) e `.prettierrc` (printWidth 100, singleQuote)
- [X] T005 [P] Criar `vitest.config.ts` com projetos `unit` (`tests/unit/**/*.test.ts`, ambiente node) e `integration` (`tests/integration/**/*.test.ts`, timeout 60 s, `fileParallelism: false` porque T056 troca a host key do contêiner compartilhado, `globalSetup` em `tests/integration/global-setup.ts`)
- [X] T006 [P] Criar `playwright.config.ts` (testDir `tests/e2e`, 1 worker, retries 0, trace on-first-retry, `globalSetup` em `tests/e2e/global-setup.ts` que reutiliza `scripts/sshd.mjs up` como T035) e helper `tests/e2e/helpers/launch.ts` que roda `_electron.launch` com variável `SHELLVAULT_DATA_DIR` apontando para diretório temporário isolado por teste
- [X] T007 Adicionar scripts em `package.json`: `dev`, `build`, `lint`, `typecheck` (`tsc -b`), `test:unit`, `test:integration`, `test:e2e`, `test:secrets-scan`, `test:sshd:up`, `test:sshd:down`, `test:sshd:rotate-key`, `seed:hosts`, `bench:sftp`, `dist` (electron-builder) e `postinstall` (`electron-builder install-app-deps` para recompilar `node-pty` se necessário)
- [X] T008 [P] Criar `electron-builder.yml` (appId `dev.shellvault.app`, productName `ShellVault`, alvo `nsis` x64, `asarUnpack` para `node_modules/node-pty/**`, sem auto-update, sem publish)
- [X] T009 [P] Criar script `scripts/check-licenses.mjs` que lê `node_modules/*/package.json` das dependências de produção e falha se alguma licença não estiver em {MIT, ISC, Apache-2.0, BSD-2-Clause, BSD-3-Clause, 0BSD} (Princípio I); adicionar `npm run check:licenses`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: ponte IPC, janela segura, aparência, kit de UI, abas de terminal e fixtures de teste

**⚠️ CRITICAL**: nenhuma história começa antes desta fase

### Contrato compartilhado

- [X] T010 [P] Criar `src/shared/ipc/result.ts` com `Result<T>` = `{ ok: true; value: T } | { ok: false; error: { code: ErrorCode; message: string } }`, helpers `ok()`/`err()`, e `src/shared/ipc/errors.ts` com o enum `ErrorCode` exatamente como em contracts/ipc-bridge.md ("ErrorCode")
- [X] T011 [P] Criar `src/shared/model/types.ts` com os tipos de data-model.md: `VaultSettings`, `Group`, `Host`, `HostInput`, `HostView`, `AuthMethod = "password" | "key" | "ask"`, `CredentialSummary`, `KnownHostKey`, `ForwardRule`, `ForwardRuleInput`, `ForwardRuleStatus`, `Settings`, `SessionKind`, `SessionState`, `Transfer`, `RemoteEntry`, `Appearance`
- [X] T012 Criar `src/shared/ipc/api.ts` com a interface TypeScript `ShellVaultApi` cobrindo todos os módulos de contracts/ipc-bridge.md (`vault`, `hosts`, `groups`, `credentials`, `sessions`, `sftp`, `forwards`, `app`), constante `IPC_CHANNELS` com nome fixo por função/evento, e declaração global `window.shellvault: ShellVaultApi` em `src/renderer/global.d.ts` (não em `src/shared`, que não tem lib DOM) (depende de T010, T011)
- [X] T013 [P] Criar `src/main/ipc/messages.ts` mapeando cada `ErrorCode` para mensagem pt-BR fixa (ex.: `VAULT_BAD_PASSWORD` → "Senha incorreta ou cofre inválido.", `AUTH_FAILED` → "A autenticação falhou.") e função `toAppError(e: unknown): AppError` que classifica erros de `ssh2`/`net`/`fs` (`ECONNREFUSED` → `CONNECT_REFUSED`, `ENOTFOUND`/`EHOSTUNREACH` → `HOST_UNREACHABLE`, timeout → `CONNECT_TIMEOUT`, `EADDRINUSE` → `PORT_IN_USE`, `All configured authentication methods failed` → `AUTH_FAILED`) e nunca copia `stack` nem mensagem crua, exceto o texto de banner/erro de autenticação do servidor
- [X] T014 [P] Teste unitário `tests/unit/ipc/messages.test.ts`: `toAppError` mapeia cada caso de T013, retorna `INTERNAL` para desconhecidos, e a mensagem nunca contém o texto de um erro com senha embutida (`new Error("senha=s3cr3t")`)

### Ponte e janela

- [X] T015 [P] Teste unitário `tests/unit/ipc/register.test.ts` (escrever antes de T016, com `ipcMain` e frames simulados): payload inválido → `INVALID_INPUT`; `senderFrame` diferente do frame principal → rejeitado sem chamar o handler; handler 🔒 com cofre bloqueado → `VAULT_LOCKED`; exceção no handler → `INTERNAL` sem `stack` nem mensagem original
- [X] T016 Criar `src/main/ipc/register.ts` com `handle(channel, schema: ZodType, fn, { requiresUnlocked })`: verifica `event.senderFrame` igual ao frame principal da janela registrada, valida com zod (`INVALID_INPUT`), checa estado do cofre via função injetada (`VAULT_LOCKED`), captura exceções com `toAppError`; e `emit(channel, payload)` para eventos (depende de T012, T013)
- [X] T017 Criar `src/preload/index.ts` expondo via `contextBridge.exposeInMainWorld('shellvault', api)` um objeto que implementa `ShellVaultApi` só com `ipcRenderer.invoke(IPC_CHANNELS.x, ...)` e `on<Evento>(handler) => unsubscribe` com `ipcRenderer.on/removeListener`; sem expor `ipcRenderer` nem `event` (depende de T012)
- [X] T018 Criar `src/main/index.ts` e `src/main/testHooks.ts`: variáveis de teste (`SHELLVAULT_DATA_DIR`, `SHELLVAULT_TEST_DIALOG_PATH`, `SHELLVAULT_FORCE_SOLID`) lidas só por `testHooks.ts` e ignoradas quando `app.isPackaged` e `SHELLVAULT_E2E !== '1'`; diretório de dados = `testHooks.dataDir ?? app.getPath('userData')` (definir com `app.setPath` antes de `ready`); `app.requestSingleInstanceLock()`; `crashReporter` não iniciado; `BrowserWindow` com `titleBarStyle: 'hidden'`, `titleBarOverlay` (cores dos tokens, altura 36 px) e `webPreferences { contextIsolation: true, sandbox: true, nodeIntegration: false, webviewTag: false, spellcheck: false, preload }`; bloquear `will-navigate`, `setWindowOpenHandler(() => ({ action: 'deny' }))`, `session.setPermissionRequestHandler` negando tudo; CSP via header `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'`; `session.defaultSession.webRequest.onBeforeRequest` cancela toda URL que não seja `file:`, `devtools:` ou o servidor de desenvolvimento do electron-vite (FR-027); expor `registerIpcModules()` chamada no `ready` — cada tarefa que cria `src/main/ipc/<módulo>.ts` acrescenta seu registro nela (depende de T016, T017)
- [X] T019 [P] Criar `src/main/settings.ts`: lê/grava `settings.json` no diretório de dados com schema zod de data-model.md ("Settings": `theme` `"system" | "light" | "dark"` padrão `"system"`; `terminalFontFamily` padrão `"Cascadia Mono, Consolas, monospace"`; `terminalFontSize` int 8–32 padrão 14; `window { width, height, x?, y?, maximized }` padrão 1200×780); arquivo inválido → padrões; gravação atômica (temp + rename); handlers `app.getUiSettings`/`app.updateUiSettings`; salvar tamanho/posição da janela ao fechar
- [X] T020 [P] Teste unitário `tests/unit/main/settings.test.ts`: padrões quando arquivo ausente/inválido, rejeita `terminalFontSize` 7 e 33, preserva campos ao aplicar patch

### Aparência (vidro e fallback)

- [X] T021 Criar `src/main/appearance.ts`: `computeAppearance()` → `{ glass, highContrast, dark }` com `glass = !nativeTheme.prefersReducedTransparency && !nativeTheme.shouldUseHighContrastColors && build Windows >= 22621` (`os.release()`); aplicar `win.setBackgroundMaterial(glass ? 'acrylic' : 'none')` e `backgroundColor` transparente/sólido; reagir a `nativeTheme.on('updated')` emitindo `app.onAppearanceChanged`; handler `app.getAppearance` (FR-031, FR-033; research R8)
- [X] T022 [P] Criar `src/renderer/theme/tokens.ts` com tokens de cor para 6 modos (`glass-dark`, `glass-light`, `solid-dark`, `solid-light`, `high-contrast` usando cores de sistema CSS `Canvas`/`CanvasText`/`Highlight`) incluindo `surfaceGlass` com opacidade mínima 0,72 (escuro) e 0,78 (claro), `glassEdge` (gradiente de borda 1 px), `terminalBackground`/`panelSolid` opacos (FR-032), texto primário/secundário; e `src/renderer/theme/apply.ts` que escreve os tokens como CSS custom properties em `:root` a partir de `Appearance` + `Settings.theme`
- [X] T023 [P] Teste unitário `tests/unit/theme/contrast.test.ts`: para cada modo de vidro, compõe `surfaceGlass` sobre fundo #FFFFFF e #000000 (alpha blending) e verifica contraste WCAG ≥ 4,5:1 para texto primário e secundário (FR-034); nos modos sólidos verifica contra a própria superfície
- [X] T024 [P] Criar `src/renderer/styles/global.css` (reset, fonte de interface `Segoe UI Variable, Segoe UI, system-ui`, classes `.glass` com fundo `var(--surface-glass)` + borda `var(--glass-edge)` + raio 10 px, `.solid` com `var(--panel-solid)`, foco visível, `prefers-reduced-motion`)

### Kit de UI e i18n

- [X] T025 [P] Criar `src/renderer/i18n/pt-BR.ts` com objeto tipado `t` (seções `common`, `vault`, `hosts`, `groups`, `credentials`, `terminal`, `sftp`, `forwarding`, `settings`, `errors`) e função `fmtBytes`, `fmtDate` (`Intl` pt-BR) (FR-029)
- [X] T026 [P] Criar componentes próprios em `src/renderer/ui/`: `Button.tsx`, `TextField.tsx` (com variante senha e botão mostrar/ocultar), `Select.tsx`, `Dialog.tsx` (modal com foco preso, Esc fecha, `role="dialog"`), `ConfirmDialog.tsx`, `Menu.tsx` (menu de contexto com teclado), `Toast.tsx` (erros na interface, FR-030), `Spinner.tsx`, todos usando classes `.glass`/`.solid`
- [X] T027 [P] Criar ícones SVG próprios em `src/renderer/ui/icons.tsx` (server, folder, folder-open, terminal, file, upload, download, lock, unlock, plus, search, close, more, key, forward, settings, warning, refresh) desenhados no projeto em grade 16×16, `stroke="currentColor"`

### Casca do app e abas de terminal (compartilhado por US2, US4, US5)

- [X] T028 Criar `src/renderer/index.html` (meta CSP igual à de T018), `src/renderer/main.tsx` (monta `<App/>`, carrega appearance + settings, aplica tema, assina `onAppearanceChanged`) e `src/renderer/app/App.tsx` com layout: barra de título própria (`titleBarStyle: 'hidden'` + `titleBarOverlay` em T018), barra lateral `.glass` (slot para hosts), área de abas `.glass` e área de conteúdo `.solid` (depende de T022, T024, T026)
- [X] T029 Criar `src/renderer/stores/sessions.ts` (Zustand): lista de `Session` (`id`, `kind`, `title`, `hostId`, `shell`, `state`, `order`) com ações `add`, `setState`, `rename`, `reorder`, `close`, `activate`; assina `sessions.onState` (FR-012)
- [X] T030 Criar `src/renderer/features/terminal/TabBar.tsx`: abas com título, indicador de estado (conectando/conectada/desconectada/encerrada), renomear com duplo clique, reordenar por arrastar, fechar, menu "novo terminal local" (FR-012)
- [X] T031 Criar `src/renderer/features/terminal/TerminalView.tsx`: instancia `Terminal` do xterm com `scrollback: 10000`, `allowProposedApi`, addons fit, unicode11 (`activeVersion = '11'`) e webgl (fallback DOM em `onContextLoss`/erro); fonte/tamanho de `Settings`; tema claro/escuro (FR-015, FR-018); `onData` → `sessions.write`; `ResizeObserver` + fit → `sessions.resize` (debounce 50 ms); `sessions.onData` filtrado por `sessionId` → `term.write`; copiar com seleção + Ctrl+Shift+C/clique direito, colar Ctrl+Shift+V/clique direito; abas inativas ficam montadas e ocultas (`display:none`) para manter o buffer, **sem** o addon WebGL — ele é carregado ao ativar a aba e descartado ao ocultá-la (Chromium limita ~16 contextos WebGL; research R4); `fit`/resize ignorados enquanto oculta e reaplicados ao reexibir (US2-5, SC-007); a sessão é aberta com o tamanho calculado pelo `fit` inicial
- [X] T032 Criar `src/main/sessions/registry.ts`: registro em memória de sessões ativas (`id`, `kind`, `write`, `resize`, `close`, `reconnect?`), batching de saída (lote a cada 8 ms ou 64 KiB → `emit(sessions.onData)`), handlers genéricos `sessions.write`, `sessions.resize` (cols/rows int 1–1000), `sessions.close`, `sessions.activeSummary` (depende de T016)

### Fixtures de teste

- [X] T033 [P] Criar `tests/fixtures/sshd/Dockerfile` (Alpine + `openssh-server` + `openssh-sftp-server` + `socat`), `tests/fixtures/sshd/sshd_config` (`PasswordAuthentication yes`, `KbdInteractiveAuthentication yes`, `AuthenticationMethods` por usuário via `Match User`, `AllowTcpForwarding yes`, `Subsystem sftp internal-sftp`), `tests/fixtures/sshd/entrypoint.sh` criando usuários `pwuser` (senha `pw-test-123`), `keyuser` (authorized_keys das chaves de teste), `kbuser` (keyboard-interactive: pacote `openssh-server-pam`, `UsePAM yes`, `Match User kbuser` com `AuthenticationMethods keyboard-interactive`, `/etc/pam.d/sshd` com `pam_unix`: o servidor envia o prompt "Password:" via keyboard-interactive e aceita `kb-test-123` — `pam_exec` não conversa com o cliente e um segundo fator TOTP exigiria pacotes extras; o caminho exercitado no app é o mesmo) e um eco TCP `socat` em `0.0.0.0:7000` só na rede interna; `tests/fixtures/sshd/compose.yml` publicando 22 → `127.0.0.1:2222` e nomeando o serviço de eco `echo`
- [X] T034 [P] Criar `scripts/gen-test-keys.mjs` que gera em `tests/fixtures/keys/generated/` (gitignored) chaves de teste via `ssh2.utils.generateKeyPairSync`: Ed25519 sem passphrase, Ed25519 com passphrase `kp-test-123`, ECDSA P-256, RSA 3072 em PEM, e um arquivo inválido; e `scripts/sshd.mjs` com comandos `up` (gera chaves, `docker compose up -d --build`, espera porta 2222), `down` e `rotate-key` (regera a host key do contêiner e reinicia o sshd)
- [X] T035 Criar `tests/integration/global-setup.ts` que verifica Docker disponível (mensagem clara "Inicie o Docker Desktop" se não), roda `scripts/sshd.mjs up` e exporta `SSHD_HOST=127.0.0.1`, `SSHD_PORT=2222` e caminhos das chaves (depende de T033, T034)

**Checkpoint**: `npm run dev` abre a janela com moldura de vidro (ou sólida no fallback), lint/typecheck/unit verdes, contêiner de teste sobe.

---

## Phase 3: User Story 1 - Cofre com senha mestra (Priority: P1) 🎯 MVP

**Goal**: criar, desbloquear, bloquear (manual e por inatividade), trocar senha e apagar o cofre cifrado.

**Independent Test**: criar o cofre, fechar, reabrir, desbloquear com a senha correta, ver a senha errada recusada e confirmar que nenhum arquivo contém segredo legível.

### Tests for User Story 1 ⚠️ (escrever primeiro, devem falhar)

- [X] T036 [P] [US1] Teste unitário `tests/unit/vault/crypto.test.ts`: `deriveKek` Argon2id com parâmetros do cabeçalho é determinística para mesma senha+salt; `seal/open` XChaCha20-Poly1305 com AD: abre com AD igual, falha com AD alterado, falha com 1 bit trocado no ciphertext; nonces diferentes a cada `seal`; `memzero` zera o buffer
- [X] T037 [P] [US1] Teste unitário `tests/unit/vault/format.test.ts`: `encode/decode` do arquivo (`magic` `SVLT`, `formatVersion` uint16 = 1, `headerLength` uint32, header JSON, payload); `decode` de magic errado → `VAULT_UNREADABLE`; `formatVersion` 2 → `VAULT_UNSUPPORTED_VERSION`; truncado → `VAULT_UNREADABLE`
- [X] T038 [P] [US1] Teste unitário `tests/unit/vault/store.test.ts` (diretório temporário, parâmetros KDF baixos via injeção): `create` com senha < 8 → `WEAK_PASSWORD`, confirmação diferente → `INVALID_INPUT`, `create` com cofre existente → `VAULT_EXISTS`; `unlock` certo/errado (`VAULT_BAD_PASSWORD`); header adulterado (ex.: `opsLimit` trocado) → `VAULT_BAD_PASSWORD`; `changePassword` faz a antiga falhar e a nova abrir com o mesmo conteúdo; `lock` zera a DEK e operações passam a dar `VAULT_LOCKED`; arquivo corrompido não é sobrescrito (bytes idênticos após tentativa) e estado vira `unreadable`; `erase` remove `vault.svlt` e `vault.bak`; gravação atômica deixa `vault.bak` com a versão anterior; após `create`, `mutate` com conteúdo de teste e `changePassword`, varrer o diretório e não encontrar a senha mestra nem trechos do conteúdo em UTF-8, UTF-16LE ou base64 (teste independente de US1)
- [X] T039 [P] [US1] Teste unitário `tests/unit/vault/autolock.test.ts` (fake timers): bloqueia após `autoLockMinutes` (padrão 15) sem atividade; `reportActivity` reinicia o timer; `null` desativa; valores fora de 1–120 rejeitados
- [X] T040 [P] [US1] Teste E2E `tests/e2e/vault.spec.ts`: primeira execução mostra criação com aviso "sem recuperação" (FR-006); criar com `senha-teste-1`; reiniciar app → tela de desbloqueio; senha errada mostra mensagem genérica; senha certa desbloqueia; bloquear manualmente → o painel de desbloqueio aparece no lugar da barra lateral; trocar senha; apagar cofre com confirmação dupla volta à criação

### Implementation for User Story 1

- [X] T041 [P] [US1] Implementar `src/main/vault/crypto.ts` sobre `libsodium-wrappers-sumo`: `ready()`, `deriveKek(password, { opsLimit, memLimit, salt })` com `crypto_pwhash` `ALG_ARGON2ID13` (padrões `OPSLIMIT_MODERATE`/`MEMLIMIT_MODERATE`), `randomKey()` 32 bytes, `seal(key, plaintext, ad)` → `nonce(24) ‖ ciphertext` com `crypto_aead_xchacha20poly1305_ietf_encrypt`, `open(key, sealed, ad)`, `memzero` (research R9)
- [X] T042 [P] [US1] Implementar `src/main/vault/format.ts`: `encodeVaultFile({ header, payload })`/`decodeVaultFile(bytes)` com layout de data-model.md ("Arquivo do cofre"); header `{ kdf: { alg: "argon2id13", opsLimit, memLimit, salt }, wrappedDek, createdAt }` validado por zod; `SUPPORTED_FORMAT_VERSION = 1`; `src/main/vault/migrations.ts` com dois registros vazios — `formatMigrations` (envelope, por `formatVersion`) e `contentMigrations` (conteúdo, por `schemaVersion`, aplicadas após decifrar e antes do zod) — e backup `vault.vN.bak` antes de gravar a versão migrada (data-model "Migração")
- [X] T043 [P] [US1] Implementar `src/main/vault/content.ts`: schema zod de `VaultContent` (`schemaVersion: 1`, `settings`, `groups`, `hosts`, `credentials`, `knownHostKeys`, `forwardRules`) e `VaultSettings` (`autoLockMinutes`: int 1–120 ou `null`, padrão 15; `connectTimeoutSeconds`: int 5–120, padrão 15); `emptyContent()`
- [X] T044 [US1] Implementar `src/main/vault/store.ts` (`VaultStore`): estados `nonexistent | locked | unlocked | unreadable`; `create(password, confirmation)` (mínimo 8 caracteres, FR-001), `unlock`, `lock` (zera DEK/KEK, descarta conteúdo), `changePassword` (re-encapsula só a DEK com novo salt, FR-004), `erase` (FR-006), `read()`/`mutate(fn)` que serializa e grava com escrita atômica (temp no mesmo diretório + `fsync` + `rename`, cópia anterior em `vault.bak`); mutações serializadas por fila; evento `stateChanged` (depende de T041–T043)
- [X] T045 [US1] Implementar `src/main/vault/autolock.ts`: timer no principal com `autoLockMinutes`, `reportActivity()` reinicia, chama `store.lock()`; lock não fecha sessões (FR-003) (depende de T044)
- [X] T046 [US1] Implementar `src/main/ipc/vault.ts`: handlers `vault.getStatus`, `create`, `unlock`, `lock`, `changePassword` 🔒, `erase` (exige `confirmPhrase === "APAGAR"`), `getSettings`/`updateSettings` 🔒, `reportActivity`, evento `onStateChanged`; injetar `store.state === 'unlocked'` em `register.ts` para as 🔒 (depende de T016, T044, T045)
- [X] T047 [P] [US1] Criar `src/renderer/stores/vault.ts` (Zustand) com `state`, ações que chamam `window.shellvault.vault.*`, assinatura de `onStateChanged`; e `src/renderer/app/activity.ts` que escuta `keydown`/`pointerdown`/`wheel` na janela (inclusive terminal) e chama `vault.reportActivity()` no máximo a cada 30 s
- [X] T048 [P] [US1] Criar `src/renderer/features/vault/CreateVaultScreen.tsx`: senha + confirmação, indicador de mínimo de 8 caracteres, aviso explícito de que não há recuperação (FR-006), erros via Toast
- [X] T049 [P] [US1] Criar `src/renderer/features/vault/UnlockScreen.tsx` (campo de senha, mensagem genérica em erro, link "Esqueci a senha" → `EraseVaultDialog`) e `src/renderer/features/vault/UnreadableScreen.tsx` (explica corrupção/versão desconhecida; só oferece apagar)
- [X] T050 [P] [US1] Criar `src/renderer/features/vault/EraseVaultDialog.tsx` com confirmação dupla (1º `ConfirmDialog`, 2º digitar `APAGAR`) e `src/renderer/features/vault/ChangePasswordDialog.tsx`
- [X] T051 [US1] Criar `src/renderer/features/settings/SettingsDialog.tsx` com seção Cofre (bloqueio automático: 1–120 min ou desativado; tempo limite de conexão 5–120 s) e seção Aparência (tema, fonte, tamanho — FR-018, usa `app.updateUiSettings`); botão de bloqueio manual na barra de título; roteamento em `App.tsx` por `vault.state`: `nonexistent` → criar, `locked` → painel de desbloqueio **no lugar da barra lateral de hosts**, mantendo `TabBar`, abas abertas (SSH, local, SFTP) e menu "Novo terminal local" visíveis e utilizáveis (FR-003, US4-3); conectar/reconectar a partir de uma aba com cofre bloqueado foca o painel de desbloqueio; `unreadable` → tela de erro (depende de T047–T050)

**Checkpoint**: US1 completa — T036–T040 verdes; cofre funciona sozinho.

---

## Phase 4: User Story 2 - Cadastrar hosts e conectar via SSH em abas (Priority: P1)

**Goal**: cadastrar hosts e credenciais (senha/chave), conectar com verificação de host key, keyboard-interactive, abas, reconexão e redimensionamento.

**Independent Test**: cadastrar host com senha e host com chave, conectar a ambos em abas separadas, rodar comandos, encerrar.

**Depende de**: US1 (hosts e credenciais vivem no cofre).

### Tests for User Story 2 ⚠️

- [X] T052 [P] [US2] Teste unitário `tests/unit/domain/hosts.test.ts`: validação de `HostInput` — `label` 1–100 caracteres aparado; `address` hostname RFC 1123, IPv4 ou IPv6 (com colchetes normalizado sem); `port` 1–65535 padrão 22; `username` 1–255 sem espaços nem controle; `authMethod` `"password" | "key" | "ask"`; `credentialId` obrigatório se `authMethod` ≠ `"ask"` e tipo da credencial combinando; `create/update/remove` atualizam `createdAt`/`updatedAt`; `remove` apaga `ForwardRule` do host e mantém credenciais
- [X] T053 [P] [US2] Teste unitário `tests/unit/domain/credentials.test.ts`: importar Ed25519/ECDSA/RSA (OpenSSH e PEM) das fixtures; passphrase errada → `KEY_BAD_PASSPHRASE` e nada salvo; arquivo inválido → `KEY_UNSUPPORTED`; deriva `keyType`, `publicKey` (formato authorized_keys) e `fingerprint` `SHA256:…`; `generateEd25519` produz chave que `parseKey` abre; `remove` em uso → `INVALID_INPUT`; `toSummary` nunca contém `privateKey`, `passphrase` nem `password`
- [X] T054 [P] [US2] Teste unitário `tests/unit/domain/knownHosts.test.ts`: `endpoint` `"<address normalizado>:<port>"`; `check()` retorna `new`, `match` ou `changed` (fingerprint diferente ou `keyType` diferente); `accept()` grava/substitui com `acceptedAt`; fingerprint calculada como `SHA256:` + base64 sem padding do blob
- [X] T055 [P] [US2] Teste de integração `tests/integration/ssh-connect.test.ts` contra o contêiner: conecta `pwuser` por senha, `keyuser` por Ed25519 com e sem passphrase, ECDSA e RSA; `kbuser` responde o prompt keyboard-interactive (`kb-test-123`); senha errada → `AUTH_FAILED`; porta fechada → `CONNECT_REFUSED`; IP não roteável com `readyTimeout` 15 s → `CONNECT_TIMEOUT`; `echo $TERM` = `xterm-256color`; resize altera `stty size`; mede clique→shell pronto < 3 s (SC-002)
- [X] T056 [P] [US2] Teste de integração `tests/integration/host-key.test.ts`: primeira conexão emite `onHostKey kind "new"` e não autentica antes de `answerHostKey(true)`; recusar → `HOST_KEY_REJECTED`; após `test:sshd:rotate-key`, conexão emite `kind "changed"` com `previousFingerprint` e o estado nunca passa a `authenticating` sem aceite (SC-005); `afterAll` restaura a host key original do contêiner
- [X] T057 [P] [US2] Teste E2E `tests/e2e/ssh.spec.ts`: criar cofre, cadastrar `pwuser` com senha, conectar, aceitar host key, digitar `echo ok` e ver saída; abrir segunda aba com `keyuser`, alternar abas mantendo conteúdo (US2-5); com a aba SSH aberta, bloquear o cofre → a aba continua visível e aceita digitação (FR-003); parar o contêiner → aba mostra desconectada e botão Reconectar (US2-6)

### Implementation for User Story 2

- [X] T058 [P] [US2] Implementar `src/main/domain/hosts.ts`: schemas zod e funções puras `createHost`, `updateHost`, `removeHost`, `listHostViews` sobre `VaultContent`, com as regras de data-model.md ("Host") citadas em T052
- [X] T059 [P] [US2] Implementar `src/main/domain/credentials.ts`: `createPassword(name, password)` (`name` 1–100, `password` 1–1024), `importKey(name, pemText, passphrase?)` com `ssh2.utils.parseKey` (aceitar `ssh-ed25519`, `ecdsa-sha2-nistp256/384/521`, `ssh-rsa`; `origin: "imported"`), `generateEd25519(name, passphrase?)` com `ssh2.utils.generateKeyPairSync('ed25519', { passphrase, cipher: 'aes256-ctr' })` (`origin: "generated"`), `rename`, `remove` (bloqueado se referenciada), `toSummary` com `usedByHostCount`
- [X] T060 [P] [US2] Implementar `src/main/domain/knownHosts.ts`: `endpointOf(address, port)`, `fingerprintOf(keyBlob)` (SHA-256 base64 sem `=`), `check(content, endpoint, keyType, fingerprint)`, `accept(...)`
- [X] T061 [US2] Implementar `src/main/ipc/hosts.ts` e `src/main/ipc/credentials.ts`: handlers 🔒 `hosts.list/create/update/remove` e `credentials.list/createPassword/importKey/pickKeyFile/generateEd25519/copyPublicKey/rename/remove` usando `store.mutate`; `pickKeyFile` abre `dialog.showOpenDialog` no principal e `importKey({ filePath })` lê o arquivo só no principal (limite 64 KiB); `copyPublicKey` usa `clipboard.writeText` (depende de T046, T058, T059)
- [X] T062 [US2] Implementar `src/main/ssh/connection.ts`: `openConnection({ host, credential, timeoutSeconds, callbacks })` com `ssh2.Client`: `readyTimeout = connectTimeoutSeconds*1000`, `keepaliveInterval 15000`, `keepaliveCountMax 3`, `tryKeyboard: true`, `hostVerifier(key, verify)` assíncrono que calcula fingerprint, consulta `knownHosts.check` e, se `new`/`changed`, chama `callbacks.onHostKey` e aguarda resposta (`accept` grava no cofre via `store.mutate`); evento `keyboard-interactive` → `callbacks.onPrompt`; `authMethod "ask"` → prompt de senha antes de conectar; erros via `toAppError`; credenciais lidas do cofre no momento da conexão e não guardadas no objeto de sessão após `ready` (depende de T060)
- [X] T063 [US2] Implementar `src/main/ssh/terminalSession.ts` e `src/main/ipc/sessions.ts`: `sessions.openSsh(hostId, { cols, rows })` 🔒 (ints 1–1000) cria sessão no registry (T032), conecta via T062, abre `client.shell({ term: 'xterm-256color', cols, rows })`, encaminha dados ao batching, `resize` → `stream.setWindow(rows, cols, 0, 0)`; estados `connecting → awaitingHostKey → authenticating → (awaitingPrompt ⇄ authenticating) → connected` e `disconnected(reason)` em `close`/erro; `sessions.reconnect` 🔒 reabre com o mesmo `sessionId`; `answerHostKey`, `answerPrompt`; eventos `onState`, `onHostKey`, `onPrompt` (depende de T032, T062)
- [X] T064 [P] [US2] Criar `src/renderer/stores/hosts.ts` (Zustand): `hosts: HostView[]`, `credentials: CredentialSummary[]`, ações CRUD chamando a ponte; recarrega ao desbloquear e limpa ao bloquear
- [X] T065 [P] [US2] Criar `src/renderer/features/hosts/HostForm.tsx` (diálogo criar/editar: rótulo, endereço, porta padrão 22, usuário, método senha/chave/perguntar; seleção ou criação inline de credencial) e `src/renderer/features/hosts/CredentialsDialog.tsx` (listar, criar senha, importar chave por arquivo ou colando texto com passphrase, gerar Ed25519 e copiar chave pública — FR-008, FR-009)
- [X] T066 [P] [US2] Criar `src/renderer/features/terminal/HostKeyDialog.tsx`: `kind "new"` mostra tipo + `SHA256:…` com Aceitar/Cancelar; `kind "changed"` mostra alerta destacado com impressão anterior e nova, botão de aceite exige marcar "Entendo o risco" (FR-013)
- [X] T067 [P] [US2] Criar `src/renderer/features/terminal/PromptDialog.tsx`: renderiza `name`, `instructions` e cada prompt (`echo: false` → campo senha) e envia `answerPrompt` (FR-014)
- [X] T068 [US2] Criar `src/renderer/features/hosts/HostList.tsx` na barra lateral (lista plana nesta fase: rótulo + endereço para diferenciar duplicados, duplo clique/Enter conecta, menu editar/excluir com confirmação) e integrar em `App.tsx`: conectar cria aba SSH (T029), estado desconectado mostra faixa "Desconectado — Reconectar" na `TerminalView` (FR-016); reconectar com cofre bloqueado pede desbloqueio primeiro (depende de T063–T067)

**Checkpoint**: US1 + US2 = menor produto utilizável.

---

## Phase 5: User Story 3 - Organizar hosts em grupos e buscar (Priority: P2)

**Goal**: grupos aninhados, mover/duplicar hosts, busca instantânea.

**Independent Test**: cadastrar 50 hosts em grupos, buscar por fragmento de nome e endereço, mover entre grupos.

**Depende de**: US2 (hosts existentes).

### Tests for User Story 3 ⚠️

- [X] T069 [P] [US3] Teste unitário `tests/unit/domain/groups.test.ts`: `name` 1–100 aparado; `parentId` deve existir; mover grupo para descendente → `GROUP_CYCLE`; excluir grupo sobe hosts (`groupId = null`) e subgrupos (`parentId = null`) para a raiz sem excluir hosts (US3-4); `sortOrder` entre irmãos; `duplicateHost` gera novo id, rótulo `"<rótulo> (cópia)"`, mesma credencial e regras copiadas com novos ids
- [X] T070 [P] [US3] Teste unitário `tests/unit/renderer/search.test.ts`: casa substring em `label`, `address`, `username` ignorando maiúsculas e acentos (`toLocaleLowerCase('pt-BR')` + NFD); com 1.000 hosts cada consulta < 200 ms (meta: < 20 ms) (SC-003)
- [X] T071 [P] [US3] Teste E2E `tests/e2e/groups.spec.ts`: criar grupo `Clientes` e subgrupo `ACME`, mover host por arrastar e por menu, buscar `ACME`/fragmento de IP filtrando enquanto digita, excluir grupo com confirmação e ver hosts na raiz, duplicar host

### Implementation for User Story 3

- [X] T072 [P] [US3] Implementar `src/main/domain/groups.ts` (`createGroup`, `renameGroup`, `moveGroup` com detecção de ciclo, `removeGroup`) e adicionar `duplicateHost`/`moveHost` em `src/main/domain/hosts.ts`, com regras de data-model.md ("Group", "Host")
- [X] T073 [US3] Implementar `src/main/ipc/groups.ts` (handlers 🔒 `groups.list/create/rename/move/remove`) e adicionar `hosts.duplicate`/`hosts.move` em `src/main/ipc/hosts.ts` (depende de T072)
- [X] T074 [P] [US3] Implementar `src/renderer/features/hosts/search.ts`: `buildIndex(hosts)` com campos normalizados e `filterHosts(index, query)`; índice recalculado quando `hosts` muda (FR-011)
- [X] T075 [US3] Evoluir `src/renderer/features/hosts/HostList.tsx` para árvore de grupos (expandir/recolher, criar/renomear/excluir grupo e subgrupo, arrastar host/grupo para grupo ou raiz, duplicar host) e criar `src/renderer/features/hosts/SearchBox.tsx` que filtra enquanto digita mostrando resultados planos com caminho do grupo; virtualizar a lista quando > 200 itens (depende de T073, T074)
- [X] T076 [P] [US3] Criar `scripts/seed-hosts.mjs` que, com `SHELLVAULT_DATA_DIR` de desenvolvimento e senha via variável `SHELLVAULT_DEV_PASSWORD`, cria N hosts distribuídos em grupos aninhados usando `VaultStore` (para `npm run seed:hosts -- 1000`)

**Checkpoint**: US3 funcional com US1/US2.

---

## Phase 6: User Story 4 - Terminal local (Priority: P2)

**Goal**: abas de PowerShell e Prompt de Comando sem depender do cofre.

**Independent Test**: abrir aba de PowerShell e de Prompt de Comando, rodar comandos, fechar.

**Depende de**: apenas Foundational (independe do cofre).

### Tests for User Story 4 ⚠️

- [X] T077 [P] [US4] Teste unitário `tests/unit/local/shells.test.ts`: `resolveShell("powershell")` = `%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`, `resolveShell("cmd")` = `%SystemRoot%\System32\cmd.exe`, valor fora de `"powershell" | "cmd"` rejeitado; nunca resolve pelo PATH
- [X] T078 [P] [US4] Teste de integração `tests/integration/local-pty.test.ts`: abre `cmd` e `powershell`, `cd` imprime `os.homedir()`, `exit` emite estado `exited(code)`, resize não lança erro
- [X] T079 [P] [US4] Teste E2E `tests/e2e/terminal-render.spec.ts` numa aba PowerShell: `Write-Host` com sequências de 256 cores e true color (`ESC[38;2;r;g;bm`) aparece colorido no buffer do xterm; emoji e CJK ocupam a largura correta (unicode11); copiar a seleção e colar reproduz o texto; após 15.000 linhas o buffer mantém ≥ 10.000 linhas roláveis (FR-015)
- [X] T080 [P] [US4] Teste E2E `tests/e2e/local-terminal.spec.ts`: com cofre bloqueado, menu "novo terminal local" → PowerShell abre e executa `echo ok`; `exit` mostra "processo encerrado" na aba (US4-2, US4-3)

### Implementation for User Story 4

- [X] T081 [P] [US4] Implementar `src/main/local/shells.ts` (`resolveShell` com caminhos absolutos de `process.env.SystemRoot`)
- [X] T082 [US4] Implementar `src/main/local/ptySession.ts` e handler `sessions.openLocal(shell, { cols, rows })` em `src/main/ipc/sessions.ts` (sem 🔒): `pty.spawn(resolveShell(shell), [], { name: 'xterm-256color', cols, rows, cwd: os.homedir(), env: process.env, useConpty: true })`, dados → batching do registry, `resize`, `onExit` → estado `exited(code)` (FR-017) (depende de T032, T081)
- [X] T083 [US4] Adicionar em `src/renderer/features/terminal/TabBar.tsx` o menu "Novo terminal local" com opções PowerShell e Prompt de Comando (atalho Ctrl+Shift+T abre PowerShell) e, em `TerminalView.tsx`, a faixa "Processo encerrado (código N)" com botão fechar aba; o menu fica disponível com cofre bloqueado (depende de T082)

**Checkpoint**: US4 funcional sozinha.

---

## Phase 7: User Story 5 - Transferir arquivos via SFTP (Priority: P3)

**Goal**: navegador remoto com envio/download, progresso, conflitos, cancelamento e operações de arquivo.

**Independent Test**: abrir SFTP, navegar, enviar e baixar arquivo de 100 MB e comparar o conteúdo.

**Depende de**: US2 (conexão SSH, host key, credenciais).

### Tests for User Story 5 ⚠️

- [X] T084 [P] [US5] Teste unitário `tests/unit/sftp/partial.test.ts`: nome temporário `<nome>.svpart-<id8>`; plano de conflito por arquivo (`replace` / `skip` / `cancelAll` cancela os restantes da mesma operação); estados `queued → running → completed | cancelled | failed | skipped`
- [X] T085 [P] [US5] Teste de integração `tests/integration/sftp.test.ts`: `list` do home retorna `name`, `size`, `modifiedAt`, `type`; `mkdir` com nome `teste ç ã` e espaços; `rename`; `remove` recursivo; upload e download de 100 MB com SHA-256 idêntico; conflito emite `onConflict` e respeita `skip`/`replace`; cancelamento no meio deixa nenhum arquivo com nome final e nenhum `.svpart-*` (local e remoto) (FR-021, US5-6)
- [X] T086 [P] [US5] Teste E2E `tests/e2e/sftp.spec.ts`: abrir SFTP de `pwuser`, entrar e sair de pasta (caminho atualiza), criar pasta, excluir com confirmação, baixar arquivo (diálogo de destino mockado via `SHELLVAULT_TEST_DIALOG_PATH`)
- [X] T087 [P] [US5] Criar `scripts/bench-sftp.mjs`: cria arquivo aleatório de 1 GB, mede upload/download pelo `src/main/sftp/transfers.ts` e por `sftp.exe` (OpenSSH do Windows) contra o contêiner, compara hashes e falha se a taxa < 80% (SC-006)

### Implementation for User Story 5

- [X] T088 [US5] Implementar `src/main/sftp/browser.ts` e handlers em `src/main/ipc/sftp.ts`: `sftp.open(hostId)` 🔒 cria sessão `kind "sftp"` usando `openConnection` (T062, mesmo fluxo de host key/prompts) e `client.sftp()`, retorna `homePath` via `realpath('.')`; `list` (ordenar pastas primeiro, mapear `type` por `attrs.mode`), `mkdir`, `rename`, `remove(path, recursive)` (FR-019, FR-022) (depende de T062, T063)
- [X] T089 [US5] Implementar `src/main/sftp/transfers.ts`: fila com até 3 transferências simultâneas, cada uma em canal SFTP próprio; upload via `fastPut` para `<remoteDir>/<nome>.svpart-<id8>` e `rename` final (se `replace`, `unlink` do existente antes); download via `fastGet` para `<destDir>/<nome>.svpart-<id8>` e `fs.rename`; `step` → progresso throttled a 10/s em `onTransfer`; detecção de conflito antes de iniciar → `onConflict` e espera `resolveConflict`; `cancel` encerra o canal e apaga o parcial; falha apaga o parcial (FR-020, FR-021); handlers `sftp.upload` (diálogo multi-seleção no principal), `sftp.download` (diálogo de pasta no principal; em teste usa `SHELLVAULT_TEST_DIALOG_PATH`), `resolveConflict`, `cancel` (depende de T088)
- [X] T090 [P] [US5] Criar `src/renderer/stores/transfers.ts` (Zustand) assinando `sftp.onTransfer` e `sftp.onConflict`
- [X] T091 [US5] Criar `src/renderer/features/sftp/SftpView.tsx` (fundo sólido, FR-032): barra de caminho clicável, botão subir, tabela nome/tamanho/data/tipo (`fmtBytes`, `fmtDate`), duplo clique entra na pasta, menu de contexto (baixar, renomear, excluir com `ConfirmDialog`, nova pasta), botões Enviar/Baixar com seleção múltipla; abrir pelo menu do host na `HostList` ("Abrir SFTP") como nova aba `kind "sftp"` (depende de T088, T090)
- [X] T092 [P] [US5] Criar `src/renderer/features/sftp/TransferPanel.tsx` (lista com barra de progresso, velocidade, cancelar) e `src/renderer/features/sftp/ConflictDialog.tsx` (Substituir / Pular / Cancelar, mostrando tamanho e data do existente) (depende de T090)

**Checkpoint**: US5 funcional.

---

## Phase 8: User Story 6 - Port forwarding local (Priority: P3)

**Goal**: regras de encaminhamento local por host, ativar/desativar com status.

**Independent Test**: regra `127.0.0.1:17000 → echo:7000` via host de teste, ativar e trocar dados pela porta local.

**Depende de**: US2 (conexão SSH).

### Tests for User Story 6 ⚠️

- [X] T093 [P] [US6] Teste unitário `tests/unit/domain/forwardRules.test.ts`: `bindAddress` padrão `127.0.0.1`; outro valor exige `allowNonLoopback: true` (FR-025); `bindPort` e `destPort` 1–65535; `bindPort` único entre regras com mesmo `bindAddress`; `hostId` deve existir; `label` opcional
- [X] T094 [P] [US6] Teste de integração `tests/integration/forwarding.test.ts`: ativar regra `127.0.0.1:17000 → echo:7000` em `pwuser` e trocar bytes por eco; status `starting → active`; porta já ocupada → `error(portInUse)` e regra não ativa (US6-3); desativar libera a porta (novo `listen` funciona); derrubar o contêiner → status `inactive` e porta liberada (FR-026); duas regras do mesmo host compartilham uma conexão
- [X] T095 [P] [US6] Teste E2E `tests/e2e/forwarding.spec.ts`: criar regra no host, ativar, ver status "ativa", desativar; tentar `0.0.0.0` exige confirmação de aviso; com uma regra ativa, bloquear o cofre → indicador de encaminhamentos ativos continua visível e permite desativar (FR-003)

### Implementation for User Story 6

- [X] T096 [P] [US6] Implementar `src/main/domain/forwardRules.ts` (`createRule`, `updateRule`, `removeRule`, `listForHost`) com as regras de data-model.md ("ForwardRule") citadas em T093
- [X] T097 [US6] Implementar `src/main/forwarding/manager.ts`: uma conexão por host (via `openConnection`, T062) aberta na primeira ativação e fechada ao desativar a última regra; por regra `net.createServer` em `bindAddress:bindPort` (`EADDRINUSE` → `error(portInUse)`), cada socket → `client.forwardOut(socket.remoteAddress, socket.remotePort, destHost, destPort)` e `pipe` bidirecional; queda da conexão fecha os servidores e emite `inactive`; status `inactive → starting → active | error(reason)` (FR-024, FR-026) (depende de T062, T096)
- [X] T098 [US6] Implementar `src/main/ipc/forwards.ts`: handlers 🔒 `forwards.list/create/update/remove/activate` e, sem 🔒, `forwards.deactivate` e `forwards.listActive` (só dados não sensíveis, contrato); evento `onStatus`; `remove` de regra ativa desativa antes (depende de T097)
- [X] T099 [US6] Criar `src/renderer/stores/forwards.ts` e `src/renderer/features/forwarding/ForwardRulesPanel.tsx` (aberto pelo menu do host "Encaminhamento de portas"): lista com status (inativa/ativa/erro + motivo), alternar ativar/desativar, formulário (endereço local padrão `127.0.0.1`, porta local, host destino, porta destino, rótulo) com aviso e confirmação para endereço diferente de loopback (FR-023, FR-025); indicador global na barra de título com o número de encaminhamentos ativos que abre a lista de `listActive` com botão Desativar, disponível com cofre bloqueado (FR-003) (depende de T098)

**Checkpoint**: todas as histórias funcionais.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: requisitos transversais, segurança e critérios de sucesso

- [X] T100 Implementar confirmação ao fechar (FR-028): em `src/main/index.ts` interceptar `close` da janela, consultar `sessions.activeSummary` + transferências + encaminhamentos ativos; se houver algo, emitir `app.onCloseRequested` e só fechar após `app.confirmClose(true)`; criar `src/renderer/app/CloseConfirmDialog.tsx` listando as contagens
- [X] T101 [P] Teste E2E `tests/e2e/close-confirm.spec.ts`: com sessão local aberta, fechar a janela mostra a confirmação; cancelar mantém o app; confirmar fecha
- [X] T102 [P] Teste E2E `tests/e2e/offline-guard.spec.ts`: o renderer executa `fetch('https://example.com')` e a requisição é cancelada pelo `webRequest` do T018; nenhuma requisição de rede sai do app durante um fluxo de criar cofre e abrir terminal local (FR-027)
- [ ] T103 [P] Criar `tests/e2e/secrets-scan.spec.ts` (script `test:secrets-scan`): executa fluxo completo (criar cofre, senha `pw-test-123`, importar chave com passphrase `kp-test-123`, conectar, SFTP, forwarding, bloquear, fechar) e varre recursivamente todo `SHELLVAULT_DATA_DIR` procurando senha mestra, senhas, passphrase e trechos da chave privada em UTF-8, UTF-16LE e base64; esperado 0 ocorrências (SC-004, FR-005)
- [ ] T104 [P] Teste E2E `tests/e2e/performance.spec.ts`: abre 20 abas (locais e SSH), mede troca de aba < 100 ms, rolagem de 10.000 linhas sem bloquear (`requestAnimationFrame` < 100 ms), nenhum evento `webglcontextlost`, e redimensionamento programático da janela por 5 s com < 5% dos quadros acima de 50 ms (`PerformanceObserver` `long-animation-frame`), com vidro ligado e com `SHELLVAULT_FORCE_SOLID=1` (SC-007, SC-009); suportar `SHELLVAULT_FORCE_SOLID` em `src/main/appearance.ts` via `testHooks`
- [ ] T105 [P] Teste E2E `tests/e2e/appearance.spec.ts`: com `SHELLVAULT_FORCE_SOLID=1` a moldura usa tokens sólidos; terminal e SFTP sempre com fundo opaco; evento de mudança de aparência troca o tema sem reiniciar (FR-032, FR-033)
- [ ] T106 Revisão de segurança (Princípio II) registrada em `docs/security-review-001.md`: conferir cada linha da tabela "Revisão de segurança" de plan.md contra o código (CSP, sandbox, sender check, validação zod em todos os handlers de `src/main/ipc/*.ts`, ausência de segredos em retornos/erros, memzero no lock, caminho absoluto dos shells, loopback padrão) e corrigir achados
- [ ] T107 [P] Teste unitário `tests/unit/ipc/no-secrets.test.ts`: percorre todos os handlers registrados com um cofre de teste populado e verifica que nenhum valor retornado contém senha, passphrase ou chave privada de teste
- [ ] T108 [P] Rodar `npm run check:licenses` e registrar dependências de produção e licenças em `docs/licenses.md` (Princípio I)
- [ ] T109 [P] Escrever `README.md` (o que é, requisitos, `npm ci`, `npm run dev`, testes com Docker, build do instalador, aviso de que não há recuperação da senha mestra)
- [ ] T110 Executar a validação de `specs/001-mvp-ssh-client/quickstart.md` de ponta a ponta (verificações automáticas, cenários manuais US1–US6, tabela de SC, incluindo SC-008 com rede externa desativada) e registrar resultados em `specs/001-mvp-ssh-client/validation.md`
- [ ] T111 Gerar instalador com `npm run dist` e validar instalação limpa em Windows 11 e Windows 10 (fallback sólido no 10)
- [X] T112 Refinamento visual "moderno escuro" (pedido do usuário): paleta violeta→azul com gradiente nos botões primários e na marca (`src/renderer/ui/Logo.tsx`), avatar colorido por host (`src/renderer/ui/Avatar.tsx`), moldura de vidro única com brilho de fundo, abas em pílula, terminal/SFTP em cartão opaco arredondado com margem interna, anel de foco suave, animações curtas (respeitando `prefers-reduced-motion`) e estado vazio do workspace; sem mudança de comportamento; contraste FR-034 e fundo opaco FR-032 preservados (testes de `tests/unit/theme`)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sem dependências
- **Foundational (Phase 2)**: depende do Setup — BLOQUEIA todas as histórias
- **US1 (Phase 3)**: depende da Foundational
- **US2 (Phase 4)**: depende de US1 (cofre guarda hosts/credenciais)
- **US3 (Phase 5)**: depende de US2 (hosts)
- **US4 (Phase 6)**: depende só da Foundational — pode rodar em paralelo com US1–US3
- **US5 (Phase 7)**: depende de US2 (`openConnection`)
- **US6 (Phase 8)**: depende de US2 (`openConnection`); independe de US5
- **Polish (Phase 9)**: depende das histórias desejadas

```text
Setup → Foundational ─┬─▶ US1 ─▶ US2 ─┬─▶ US3
                      │               ├─▶ US5
                      │               └─▶ US6
                      └─▶ US4
                                       (todas) ─▶ Polish
```

### Within Each User Story

- Testes primeiro, falhando (Princípio IV)
- Domínio (`src/main/domain`, `src/main/vault`) → serviços (`ssh`, `sftp`, `forwarding`, `local`) → handlers IPC → stores → telas
- História completa antes da próxima prioridade

### Parallel Opportunities

- Setup: T003, T004, T005, T006, T008, T009 em paralelo após T001/T002
- Foundational: T010, T011, T013, T014, T019–T027, T033, T034 em paralelo; T012 → T016/T017 → T018; T028 → T029–T031
- US4 inteira em paralelo com US1–US3 (outra pessoa)
- US5 e US6 em paralelo depois de US2
- Dentro de cada história: todos os testes [P] juntos; módulos de domínio [P] juntos; telas [P] juntas

---

## Parallel Example: User Story 2

```text
# Testes (juntos, antes da implementação):
Task: "T052 Teste unitário tests/unit/domain/hosts.test.ts"
Task: "T053 Teste unitário tests/unit/domain/credentials.test.ts"
Task: "T054 Teste unitário tests/unit/domain/knownHosts.test.ts"
Task: "T055 Teste de integração tests/integration/ssh-connect.test.ts"
Task: "T056 Teste de integração tests/integration/host-key.test.ts"

# Domínio (juntos):
Task: "T058 src/main/domain/hosts.ts"
Task: "T059 src/main/domain/credentials.ts"
Task: "T060 src/main/domain/knownHosts.ts"

# Telas (juntas, após T063):
Task: "T065 HostForm.tsx + CredentialsDialog.tsx"
Task: "T066 HostKeyDialog.tsx"
Task: "T067 PromptDialog.tsx"
```

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Phase 1 Setup → Phase 2 Foundational
2. Phase 3 US1 (cofre) → validar sozinho
3. Phase 4 US2 (hosts + SSH) → **STOP and VALIDATE**: menor produto utilizável (cofre + conexão SSH em abas)

US1 sozinha é testável, mas não é útil ao usuário final; o MVP entregável é US1 + US2.

### Incremental Delivery

1. Setup + Foundational
2. US1 → US2 → demo (MVP)
3. US4 (terminal local) — pode ser feito antes ou em paralelo
4. US3 (grupos/busca)
5. US5 (SFTP) e US6 (forwarding)
6. Polish (segurança, SC, instalador)

---

## Notes

- [P] = arquivos diferentes, sem dependência pendente
- Commits pequenos em Conventional Commits (`feat:`, `test:`, `chore:`…) por tarefa ou grupo lógico
- Parar em cada checkpoint para validar a história isoladamente
- Integração exige Docker Desktop em execução (`npm run test:sshd:up`)
