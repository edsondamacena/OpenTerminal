---
description: "Tarefas da migração do ShellVault para Go + Wails (Fase 1)"
---

# Tasks: Migração do ShellVault para Go + Wails (Fase 1)

**Input**: `specs/002-wails-migration/` (plan.md, spec.md, research.md, data-model.md, contracts/bridge.md, quickstart.md)

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: obrigatórios (Constituição, Princípio IV): testes escritos antes da implementação e falhando antes de passar; cada um dos 4 bugs da versão Electron ganha teste de regressão.

**Organization**: por história de usuário (spec.md). Uma tarefa só está concluída com `go vet`, `go test`, lint, typecheck e os testes do frontend passando.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos diferentes, sem dependência pendente)
- **[Story]**: história da spec (US1…US6)
- Caminhos relativos à raiz do repositório (layout em plan.md → Project Structure)

## Checkpoints do usuário

Tarefas marcadas **⏸ usuário** exigem uma escolha do usuário (regra do skill `brand-kit`: o
usuário escolhe em cada estágio) ou acesso que só ele tem. Ao chegar nelas, parar, mostrar as
opções e esperar a resposta; as demais tarefas podem seguir em paralelo.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: ferramentas, scaffold Wails, frontend movido e servidores de teste

- [X] T001 Instalar e verificar toolchain: Go 1.26+ (1.27 se já estável), Wails CLI `v2.17.0` (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`), NSIS; `wails doctor` sem erros; registrar versões efetivas em `specs/002-wails-migration/research.md` (R1, R2)
- [X] T002 Emendar a constituição para 1.0.1 (PATCH, research R20) em `.specify/memory/constitution.md`: exemplos de linguagem passam a "Go e TypeScript `strict`" e de bibliotecas a "`golang.org/x/crypto/ssh`, `pkg/sftp`, `go-pty`"; Sync Impact Report atualizado; nenhum princípio muda de sentido
- [X] T003 Criar o scaffold Wails na raiz: `wails.json` (`name: "ShellVault"`, `outputfilename: "shellvault"`, `frontend:dir: "frontend"`, `frontend:install`/`build`/`dev:watcher` com npm), `go.mod` (`module shellvault`, `go 1.26`), `main.go` mínimo abrindo a janela; acrescentar `build/bin/`, `frontend/dist/`, `frontend/node_modules/` ao `.gitignore`
- [X] T004 Mover a interface da 001 com histórico: `git mv src/renderer frontend/src` e `git mv src/shared frontend/src/shared`; criar `frontend/package.json` (React 19, Zustand 5, `@xterm/xterm` 6 e addons fit/webgl/unicode11/search/web-links, `@floating-ui/react-dom`; dev: Vite, TypeScript 6, ESLint, Prettier, Vitest), `frontend/vite.config.ts` (saída `frontend/dist`, alias `@shared`), `frontend/tsconfig.json` (`strict`), `frontend/eslint.config.js` e `frontend/vitest.config.ts` adaptados das configs da raiz
- [X] T005 Ajustar `frontend/index.html` com a CSP de research R10 (`default-src 'self'; script-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:`) e remover dependências do Electron do frontend (`frontend/src/global.d.ts` passa a tipar o adaptador da T014)
- [X] T006 [P] Criar `scripts/check.mjs` (raiz) que roda em sequência `go vet ./...`, `go test ./internal/...`, `npm --prefix frontend run lint`, `typecheck` e `test`, falhando no primeiro erro; documentar no `package.json` da raiz como `check:wails`
- [X] T007 [P] Criar `tests/fixtures/sshd-legacy/` (Dockerfile OpenSSH + `sshd_config` com `KexAlgorithms diffie-hellman-group14-sha1,diffie-hellman-group1-sha1`, `Ciphers aes128-cbc,3des-cbc`, `HostKeyAlgorithms ssh-rsa`, `PubkeyAcceptedAlgorithms ssh-rsa`, usuário `legacyuser`/`legacy-test-123`) exposto na porta 2223 em `tests/fixtures/sshd/compose.yml`
- [X] T008 [P] Criar `tests/fixtures/idle-killer/` (contêiner `socat -T 60 TCP-LISTEN:22,fork TCP:sshd:22`) exposto na porta 2224 em `tests/fixtures/sshd/compose.yml`
- [X] T009 Atualizar `scripts/sshd.mjs` e `scripts/sshd.d.mts` para subir e aguardar `sshd`, `echo`, `sshd-legacy` e `idle-killer`, exportando `SSHD_LEGACY` e `IDLE_KILLER` (host/porta)
- [X] T010 [P] Criar `scripts/gen-vault-001.mjs` que usa o `VaultStore` da 001 (Vite `ssrLoadModule`, como `scripts/seed-hosts.mjs`) com `TEST_KDF` para gerar `tests/fixtures/vault-001/vault.svlt`, `settings.json` e `expected.json` (2 grupos aninhados, 3 hosts — um com `authMethod: "ask"` —, 1 senha, 1 chave Ed25519 com passphrase, 1 chave RSA, 2 host keys, 2 regras de encaminhamento, senha mestra `senha-teste-1`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: ponte, janela, harness de testes — bloqueia todas as histórias

- [X] T011 Escrever `internal/apperr/apperr_test.go` e implementar `internal/apperr/` (códigos usados também pelo domínio; ver contracts/bridge.md: métodos `(T, error)` com erro sanitizado em JSON, adaptador monta o `Result`): todos os `ErrorCode` da 001 + `NO_COMMON_ALGORITHM`, `AGENT_UNAVAILABLE`, `SCHEME_INVALID`, `IN_USE`, `VAULT_MIGRATION_FAILED` (contracts/bridge.md), mensagens pt-BR portadas de `src/main/ipc/messages.ts`; nenhum erro Go cru chega ao JS
- [X] T012 [P] Escrever `internal/validate/validate_test.go` e implementar `internal/validate/validate.go` (uuid v4, texto aparado com limites, inteiros com faixa, enum) → `INVALID_INPUT`
- [X] T013 Escrever `internal/bridge/contract_test.go`: por reflexão, os métodos exportados de `VaultService`, `HostService`, `CredentialService`, `SessionService`, `SftpService`, `ForwardService`, `ProfileService`, `AppService` são exatamente os de `contracts/bridge.md` (lista mantida no teste)
- [X] T014 Criar `frontend/src/bridge/index.ts`: adaptador sobre `frontend/wailsjs/go/*` que expõe a mesma forma `ShellVaultApi` de `frontend/src/shared/ipc/api.ts` (`Result<T>`) e eventos com **um** `EventsOn` por nome na inicialização, despachando por ids para assinantes (`Unsubscribe` só remove do mapa); `frontend/src/main.tsx` instala o adaptador antes de renderizar
- [X] T015 Implementar `main.go` + `internal/appwin/datadir.go`: diretório de dados `%APPDATA%\ShellVault` (`SHELLVAULT_DATA_DIR` sobrescreve, para testes), `--disable-breakpad --disable-crash-reporter` via patch local do `go-webview2` (`third_party/go-webview2/PATCH.md`; o loader ignora `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`) antes de `wails.Run`, `WebviewUserDataPath` dentro do diretório de dados, `Frameless: true`, Windows `WebviewIsTransparent`, `WindowIsTranslucent`, `BackdropType: Mica`, `SingleInstanceLock{UniqueId, OnSecondInstanceLaunch: restaurar e focar}`, logger nível `ERROR` sem arquivo, `OnBeforeClose` ligado à confirmação de fechar, `Bind` só dos 8 serviços
- [X] T016 Implementar `internal/bridge/testsvc/testsvc.go` e `main_e2e.go` (ambos `//go:build e2e`): `TestService` (`Maximise`, `Restore`, `SetSize`, `Goroutines`, `SessionCount`, `ServerSizeOf`) e `--remote-debugging-port=9333` (+ `--force-device-scale-factor` de `SHELLVAULT_E2E_SCALE`) acrescentados aos argumentos do WebView2
- [X] T017 Criar o harness E2E em `frontend/tests/e2e/helpers/launch.ts` (compila uma vez com `wails build -tags e2e`, inicia o exe com `SHELLVAULT_DATA_DIR` temporário, `chromium.connectOverCDP`, `app.close()` encerra o processo sem passar pela confirmação) e `frontend/tests/e2e/helpers/ui.ts` portado de `tests/e2e/helpers/ui.ts`; `frontend/playwright.config.ts`
- [X] T018 Criar `scripts/check-release.mjs`: compila sem tags e falha se o binário contiver `remote-debugging-port` ou `TestService` (research R18)

**Checkpoint**: `wails dev` abre a janela com a interface da 001 renderizando (sem backend ainda) e o harness E2E conecta

---

## Phase 3: User Story 1 - Paridade do núcleo: cofre, hosts e sessões SSH sem regressões (Priority: P1) 🎯 MVP

**Goal**: cofre (com migração da 001), hosts, grupos, credenciais, SSH e shell local em abas, prompt único, instância única, fechar sem vazamentos

**Independent Test**: criar cofre, cadastrar host, conectar, ver um único prompt, fechar a aba; 100 ciclos sem crescimento de goroutines; abrir cofre da 001 e ver os mesmos dados

### Tests for User Story 1 ⚠️ (escrever antes e ver falhar)

- [X] T019 [P] [US1] `internal/vault/crypto_test.go`: `argon2.IDKey` com `opsLimit/memLimit` do libsodium (`memLimit/1024` KiB, paralelismo 1) gera a mesma KEK que a 001 para um vetor gerado pela T010; seal/open com AD; AD trocado, nonce ou texto adulterado → erro
- [X] T020 [P] [US1] `internal/vault/store_test.go`: estados `nonexistent`/`locked`/`unlocked`/`unreadable`/`migratable`; criar (senha ≥ 8, confirmação igual → senão `WEAK_PASSWORD`/`INVALID_INPUT`), desbloquear (senha errada → `VAULT_BAD_PASSWORD`), bloquear zera DEK/KEK, trocar senha re-embrulha a DEK sem recifrar linhas, apagar; trocar o `sealed` de duas linhas → falha de autenticação; `PRAGMA secure_delete` ligado; o arquivo `.db` não contém em claro nenhum rótulo, endereço ou usuário de teste
- [X] T021 [P] [US1] `internal/vault/migrate001/migrate001_test.go` com `tests/fixtures/vault-001/`: migração preserva campo a campo `expected.json` (mesmos UUIDs, exceto `known_hosts`); senha errada não cria o cofre; falha simulada após gravar deixa `vault.svlt` intacto e o estado em `migratable` (o banco fica sempre aberto por causa das preferências; a importação é uma transação única); `formatVersion` > 1 → `unreadable` sem tocar o arquivo; `settings.json` vira preferências e perfil global (fonte, tamanho)
- [X] T022 [P] [US1] Portar para Go os testes de `tests/unit/domain/hosts.test.ts`, `groups.test.ts`, `credentials.test.ts`, `knownHosts.test.ts` em `internal/domain/*_test.go`, acrescentando: `authMethod: "agent"` exige `credentialId` null; `legacyAlgorithms` padrão `false`; `profileId` preservado no update (a referência a `terminal_profiles` com `is_global = false` é validada em `ProfileService.SetHostOverride`); duplicar copia `legacyAlgorithms` e `profileId`
- [X] T023 [P] [US1] `internal/session/aggregator_test.go`: lotes de no máximo 8 ms ou 32 KiB, ordem preservada, nenhum byte perdido sob 10 MiB contínuos; caractere UTF-8 partido entre leituras chega inteiro; bytes inválidos viram U+FFFD
- [X] T024 [P] [US1] `internal/session/manager_test.go` com transporte falso: 100 aberturas/fechamentos com `goleak.VerifyNone` e `runtime.NumGoroutine` estável; nenhum evento depois de `term:closed`; o PTY remoto só é pedido no primeiro `Resize` com `cols > 0 && rows > 0`; `Resize` igual ao anterior é ignorado; `Close` durante autenticação cancela sem pendurar (regressão do bug 2)
- [X] T025 [US1] `internal/sshx/ssh_integration_test.go` (`//go:build integration`, contêiner `sshd`): senha, chave (com passphrase), keyboard-interactive; host key nova pede confirmação e mudada bloqueia (`HOST_KEY_REJECTED`); prompt único: em 20 conexões de `pwuser`, a saída dos primeiros 2 s contém o prompt exatamente uma vez; fechar a sessão encerra o processo no servidor (`pgrep -u pwuser bash` vazio)
- [X] T026 [P] [US1] `internal/localpty/pty_integration_test.go`: PowerShell e cmd por caminho absoluto em `%SystemRoot%\System32`; eco de comando; código de saída; `Close` encerra o processo filho; nenhum processo iniciado aponta para `os.Executable()` (regressão do bug 1)

### Implementation for User Story 1

- [X] T027 [US1] Implementar `internal/vault/crypto.go` (Argon2id, XChaCha20-Poly1305, `nonce(24) ‖ ct`, zeragem de chaves) para passar a T019
- [X] T028 [US1] Implementar `internal/vault/schema.go`: migração `user_version` 1 com `vault_meta`, `groups`, `hosts`, `credentials`, `known_hosts`, `forward_rules`, `vault_settings`, `preferences`, `terminal_profiles`, `color_schemes` conforme data-model.md; pragmas WAL, `synchronous=FULL`, `foreign_keys=ON`, `secure_delete=ON`
- [X] T029 [US1] Implementar `internal/vault/store.go` (criar, desbloquear decifrando tudo para o índice em memória, bloquear com checkpoint do WAL e zeragem, trocar senha, apagar, CRUD transacional por linha com AD `"<tabela>:<id>"`) para passar a T020
- [X] T030 [US1] Portar `src/main/vault/autolock.ts` para `internal/vault/autolock.go` ("1–120; `null` = desativado; padrão 15") com teste portado de `tests/unit/vault/autolock.test.ts`
- [X] T031 [US1] Implementar `internal/vault/migrate001/` (decodificação `SVLT` v1, AD do embrulho reproduzindo byte a byte o `JSON.stringify` de `{v:1,kdf,createdAt}`, conteúdo `schemaVersion` 1, importação numa transação, conferência do conteúdo lido de volta, renomeação para `vault.svlt.migrated`) para passar a T021
- [X] T032 [US1] Implementar `internal/domain/hosts.go`, `groups.go`, `credentials.go` (`ssh.ParseRawPrivateKeyWithPassphrase`, geração Ed25519 e `ssh.MarshalPrivateKey`, `publicKey` em formato `authorized_keys`, `fingerprint` `SHA256:…`), `knownhosts.go` para passar a T022
- [X] T033 [US1] Implementar `internal/session/aggregator.go` e `utf8.go` para passar a T023
- [X] T034 [US1] Implementar `internal/session/manager.go` (`map[id]*Session` com mutex, `context.WithCancel`, `sync.WaitGroup`, uma goroutine de leitura por sessão, estados do data-model, `term:closed` como último evento) para passar a T024
- [X] T035 [US1] Implementar `internal/sshx/dial.go`, `auth.go`, `hostkey.go` e `interactions.go` (respostas de host key e prompts por canal ligado ao contexto da sessão; `connectTimeoutSeconds` "5–120; padrão 15") para passar a T025
- [X] T036 [P] [US1] Implementar `internal/localpty/pty.go` (interface `PTY`) com `github.com/aymanbagabas/go-pty` e `internal/localpty/shells.go` para passar a T026
- [X] T037 [US1] Implementar em `internal/bridge/` os serviços `VaultService`, `HostService`, `CredentialService` e `SessionService` (métodos e eventos `term:*` de contracts/bridge.md; `OpenSsh`/`OpenLocal` sem tamanho; `SendKeys`) e registrar no `main.go`; T013 passa para esses quatro
- [X] T038 [US1] Implementar `internal/appwin/closeguard.go` + `AppService` (`GetAppearance`, `GetPreferences`/`UpdatePreferences`, `ConfirmClose`, `WindowMinimise`/`WindowToggleMaximise`/`WindowClose`) com teste portado de `tests/unit/main/closeGuard.test.ts`
- [X] T039 [US1] Adaptar `frontend/src/stores/sessions.ts`, `frontend/src/features/terminal/dataBus.ts` e `TerminalView.tsx`: abrir sessão sem tamanho, primeiro `Resize` só após `fit()` com dimensões > 0, nenhum assinante por sessão no bridge (só o `dataBus` global), limpeza no `term:closed`
- [X] T040 [US1] Estado `migratable` na tela de desbloqueio (`frontend/src/features/vault/UnlockScreen.tsx`, textos em `frontend/src/i18n/pt-BR.ts`: "Encontramos o cofre da versão anterior. Digite a senha mestra para atualizá-lo.") e erro `VAULT_MIGRATION_FAILED`
- [X] T041 [US1] Portar para `frontend/tests/e2e/` os E2E da 001 `vault.spec.ts`, `ssh.spec.ts`, `groups.spec.ts`, `local-terminal.spec.ts`, `terminal-render.spec.ts`, `close-confirm.spec.ts`, `offline-guard.spec.ts`, e acrescentar `process.spec.ts` (segunda execução não abre janela e foca a existente; após abrir shells e sessões, uma única janela e nenhum processo `shellvault.exe` extra)
- [X] T042 [US1] E2E `frontend/tests/e2e/migration.spec.ts`: copia `tests/fixtures/vault-001/` para o diretório de dados, desbloqueia, vê todos os hosts/grupos/credenciais de `expected.json`, conecta num host com host key já conhecida sem nova confirmação (a fixture ganhou o host `Sshd de teste` em `127.0.0.1:2222` e a host key fixa de `tests/fixtures/sshd/ssh_host_ed25519_key`, só de teste)

**Checkpoint**: MVP — núcleo em Wails com paridade de cofre, hosts e terminal

---

## Phase 4: User Story 2 - Janela e layout estáveis em qualquer tamanho (Priority: P1)

**Goal**: interface sempre preenchendo a janela; terminal reajustado; menus ancorados; barra de título própria

**Independent Test**: 20 ciclos maximizar/restaurar/redimensionar em 100/125/150% sem área vazia; menus sempre inteiros

### Tests for User Story 2 ⚠️

- [X] T043 [P] [US2] E2E `frontend/tests/e2e/layout.spec.ts`: com duas abas, 20 ciclos `Maximise`/`Restore`/`SetSize` (TestService) em `SHELLVAULT_E2E_SCALE` 1, 1.25 e 1.5; após cada ciclo, o retângulo do app cobre a viewport (pontos dos quatro cantos pertencem ao app), o contêiner do terminal tem altura > 0 e o canvas/linhas do xterm ocupam o contêiner, e `ServerSizeOf(sessionId)` = `cols×rows` do xterm (regressão do bug 3); arrastar a borda gera no máximo 5 resizes no servidor
- [X] T044 [P] [US2] E2E `frontend/tests/e2e/menus.spec.ts`: no tamanho mínimo da janela, o menu do "+" abre com o topo a ≤ 8 px da base do botão e inteiro na viewport, acima do terminal (`elementFromPoint` no centro do menu é o menu) (regressão do bug 4); menu de contexto de host aberto perto dos quatro cantos fica inteiro; Esc fecha o diálogo de encaminhamento mesmo depois de o botão focado sumir (regressão B2); após aceitar a host key, digitar vai para o terminal sem clicar (regressão B3)
- [X] T045 [P] [US2] Teste em `frontend/src/features/hosts/tree.test.ts`: hosts da raiz depois de um grupo expandido têm o mesmo recuo dos grupos da raiz (regressão B4)

### Implementation for User Story 2

- [X] T046 [US2] Reescrever `frontend/src/app/app.module.css` e `frontend/src/styles/global.css`: raiz `height: 100vh` com grid `sidebar | main`, `min-height: 0`/`min-width: 0` em todos os filhos flex/grid; `.glass-edge` deixa de definir `position` (borda via `::before` em elemento já posicionado pelo próprio componente)
- [X] T047 [US2] `frontend/src/features/terminal/TerminalView.tsx`: `ResizeObserver` no contêiner → `fit()` com debounce de 50 ms → `Resize` só com dimensões > 0 e diferentes das últimas; ouvinte de `matchMedia('(resolution: …dppx)')` refaz o fit ao trocar de monitor
- [X] T048 [US2] Criar `frontend/src/ui/Popover.tsx` sobre `@floating-ui/react-dom` (`flip`, `shift`, `size`, portal no `body`, `z-index` do token de camada de menu) e reescrever `frontend/src/ui/Menu.tsx` ancorado a um elemento ou a um ponto; atualizar usos em `TabBar.tsx`, `HostList.tsx`, `SftpView.tsx`
- [X] T049 [US2] Criar `frontend/src/app/TitleBar.tsx`: área de arrasto (`--wails-draggable: drag`), botões minimizar, maximizar/restaurar (ícone conforme estado) e fechar chamando `AppService`, duplo clique alterna maximizar; substituir o cabeçalho de `App.tsx`
- [X] T050 [US2] `frontend/src/ui/Dialog.tsx`: pilha de diálogos com Esc tratado no `document` para o diálogo do topo (corrige B2) e alvo de foco ao fechar configurável; `HostKeyDialog`/`PromptDialog` devolvem o foco ao terminal da sessão (corrige B3)
- [X] T051 [US2] Corrigir o recuo em `frontend/src/features/hosts/HostList.tsx` e `tree.ts` (corrige B4) para passar a T045

**Checkpoint**: US1 + US2 — app estável em qualquer tamanho

---

## Phase 5: User Story 3 - Uso em NOC: equipamentos legados, conexões longas e atalhos (Priority: P1)

**Goal**: algoritmos legados por host, keepalive, agente, atalhos, paleta, busca, colagem segura, links

**Independent Test**: host legado conecta só com a opção; sessão sobrevive ao `idle-killer`; operação completa só por teclado

### Tests for User Story 3 ⚠️

- [X] T052 [P] [US3] `internal/sshx/algorithms_test.go`: opção desligada = listas padrão da biblioteca; ligada acrescenta `diffie-hellman-group14-sha1`, `diffie-hellman-group1-sha1`, `aes128-cbc`, `3des-cbc`, `ssh-rsa` **depois** dos modernos; chave RSA do usuário assina também com `ssh-rsa`; erro `no common algorithm` → `NO_COMMON_ALGORITHM`
- [X] T053 [P] [US3] `internal/sshx/keepalive_test.go` com conexão falsa: envia `keepalive@openssh.com` a cada `keepaliveSeconds` ("10–300, padrão 30"); 3 falhas seguidas fecham a conexão com `SESSION_CLOSED`; para ao fechar a sessão (goleak)
- [X] T054 [US3] `internal/sshx/noc_integration_test.go` (`integration`): `sshd-legacy` falha com a opção desligada (`NO_COMMON_ALGORITHM`) e conecta com ela ligada; via `idle-killer` (60 s), sessão com keepalive de 30 s continua viva após 150 s e sem keepalive cai; agente: keyring de `x/crypto/ssh/agent` servido num named pipe de teste (`SHELLVAULT_AGENT_PIPE`) autentica `keyuser`; pipe inexistente → `AGENT_UNAVAILABLE`
- [X] T055 [P] [US3] Testes de frontend: `frontend/src/features/terminal/paste.test.ts` (1 linha com quebra final não pede confirmação; várias linhas pedem, com contagem e caracteres de controle escapados), `frontend/src/features/palette/palette.test.ts` (filtro por substring sem acento sobre hosts e ações), `frontend/src/app/shortcuts.test.ts` (mapa de atalhos de FR-140)

### Implementation for User Story 3

- [X] T056 [US3] Implementar `internal/sshx/algorithms.go` e o mapeamento de erro em `internal/bridge/result.go` para passar a T052
- [X] T057 [US3] Implementar `internal/sshx/keepalive.go`, ligar ao dial e acrescentar `keepaliveSeconds` a `vault_settings` e ao diálogo de configurações (`frontend/src/features/settings/SettingsDialog.tsx`) para passar a T053
- [X] T058 [US3] Implementar `internal/sshx/agent.go` (`go-winio`, pipe `\\.\pipe\openssh-ssh-agent`, `SHELLVAULT_AGENT_PIPE` só para testes) e o método `agent` na autenticação para passar a T054
- [X] T059 [US3] `frontend/src/features/hosts/HostForm.tsx`: opção "Algoritmos legados (inseguros)" com texto de aviso, método "Agente SSH" (esconde credencial); selo de aviso para hosts legados em `HostList.tsx`; textos em `pt-BR.ts`
- [X] T060 [US3] Criar `frontend/src/app/shortcuts.ts` (captura antes do xterm: Ctrl+T, Ctrl+W com confirmação se conectada, Ctrl+Tab, Ctrl+Shift+Tab, Ctrl+Shift+F, Ctrl+K, Ctrl+=, Ctrl+-, Ctrl+0, Ctrl+Shift+T) e o item de menu da aba "Enviar combinação de teclas" → `SessionService.SendKeys`
- [X] T061 [US3] Criar `frontend/src/features/palette/CommandPalette.tsx` e `palette.ts`: conectar em host, abrir SFTP, shell local, configurações, bloquear cofre, alternar tema, busca no terminal
- [X] T062 [US3] Criar `frontend/src/features/terminal/SearchBar.tsx` com o addon `search` (destaque, anterior/próximo, contagem, maiúsculas, regex)
- [X] T063 [US3] Copiar ao selecionar; colar com Ctrl+Shift+V e botão direito; `frontend/src/features/terminal/PasteConfirmDialog.tsx` + `paste.ts`; addon `web-links` com Ctrl+clique → `AppService.OpenExternalUrl` (Go aceita só `http`/`https`)
- [X] T064 [US3] E2E `frontend/tests/e2e/noc.spec.ts`: atalhos de FR-140, paleta conectando num host só pelo teclado, busca com regex, colagem de 5 linhas pede confirmação, mensagem sugerindo "algoritmos legados" ao falhar no `sshd-legacy`

**Checkpoint**: todas as histórias P1 prontas

---

## Phase 6: User Story 4 - Personalizar o terminal com preview ao vivo (Priority: P2)

**Goal**: perfis global e por host, esquemas com presets, importação/exportação, fontes, preview

**Independent Test**: mudar fonte/tamanho/esquema com abas abertas sem reconectar; importar WT e `.itermcolors`; override por host

### Tests for User Story 4 ⚠️

- [X] T065 [P] [US4] `internal/schemes/import_test.go` com `tests/fixtures/schemes/` (esquema WT avulso, `settings.json` do WT com lista `schemes`, `.itermcolors` sRGB e P3, arquivo com cores faltando, arquivos inválidos): cores corretas, `completed` lista as preenchidas, `SCHEME_INVALID` com motivo, exportar e reimportar dá resultado idêntico
- [X] T066 [P] [US4] `internal/domain/profiles_test.go`: faixas do data-model ("8–32, padrão 14", "1.0–2.0, padrão 1.2", "100–100.000, padrão 10.000", "0.5–1.0, padrão 1.0", "0–40 px", "0–32 px, padrão 12", pesos "100–900"); exatamente um perfil global; override cria cópia completa e remover zera `profileId`; esquema `builtin` não editável; excluir esquema em uso → `IN_USE`
- [X] T067 [P] [US4] `internal/fonts/fonts_test.go`: Consolas aparece como monoespaçada e Arial não; resultado inclui as embutidas com `source: "bundled"`

### Implementation for User Story 4

- [X] T068 [US4] Implementar `internal/schemes/presets.go` (Dracula, Nord, Solarized Dark, Solarized Light, One Dark, Tokyo Night, Catppuccin + padrão da marca), `import_wt.go`, `import_iterm.go`, `export.go` para passar a T065; origem e licença de cada preset em `docs/licenses.md`
- [X] T069 [US4] Implementar `internal/domain/profiles.go` e as tabelas `terminal_profiles`/`color_schemes` no store, semeando perfil global e presets na primeira execução, para passar a T066
- [X] T070 [P] [US4] Implementar `internal/fonts/fonts.go` (`%WINDIR%\Fonts`, `%LOCALAPPDATA%\Microsoft\Windows\Fonts`, `sfnt`, cache) para passar a T067
- [X] T071 [US4] Implementar `ProfileService` em `internal/bridge/profiles.go` (métodos de contracts/bridge.md, diálogos nativos de importar/exportar, evento `profiles:changed`); T013 passa para ele
- [X] T072 [P] [US4] Adicionar JetBrains Mono e Cascadia Code (`woff2`, OFL-1.1) em `frontend/src/assets/fonts/` com `@font-face` em `frontend/src/styles/fonts.css`
- [X] T073 [US4] Criar `frontend/src/features/settings/TerminalSettings.tsx` (terminal de amostra ao vivo, controles de FR-152 só por seleção (FR-158a: galeria de esquemas com prévia, lista de fontes desenhadas na própria fonte, controles deslizantes/opções, cursor ilustrado), editor de esquema com seletores de cor, importar/exportar/duplicar, aviso ao reduzir opacidade) acessível pelas configurações e pela paleta
- [X] T074 [US4] Criar `frontend/src/features/terminal/profile.ts`: perfil efetivo aplicado a `term.options` + `fit()`, reação a `profiles:changed` em todas as abas, zoom Ctrl+=/−/0 só no `fontSize` efetivo da aba, `allowTransparency` só com opacidade < 1
- [X] T075 [US4] Override por host em `HostForm.tsx` ("Perfil de terminal próprio" abre `TerminalSettings` no escopo do host)
- [X] T076 [US4] E2E `frontend/tests/e2e/personalization.spec.ts`: com 3 abas, fonte/tamanho/esquema aplicados em < 1 s com o mesmo `sessionId` (sem reconexão); importar WT e `.itermcolors` via `SHELLVAULT_TEST_DIALOG_PATH`; exportar e reimportar; override de host só na aba desse host

**Checkpoint**: personalização completa

---

## Phase 7: User Story 5 - Nova identidade visual glass (Priority: P2)

**Goal**: marca própria (skill `brand-kit`) aplicada por tokens a todas as telas, com vidro e fallback

**Independent Test**: capturas de todas as telas nos temas escuro/claro, com e sem vidro, batem com o brand book; contraste ≥ 4,5:1; nenhum valor visual avulso

- [X] T077 [US5] ⏸ usuário — Estágio 0 do `brand-kit`: confirmar o briefing já levantado e gravar em `brand/brief.md` (`meta.lang: pt-BR`)
- [X] T078 [US5] ⏸ usuário — Estágio 1: rodadas de conceitos (símbolo terminal + cofre, clima moderno vibrante, sem o `>_` isolado) em `brand/explore/`; o usuário escolhe a direção
- [X] T079 [US5] ⏸ usuário — Estágio 2: logo final e kit (`make_kit.py`) em `brand/kit/` incluindo `favicon.ico` e PNGs; o usuário aprova
- [X] T080 [US5] ⏸ usuário — Estágio 4: `brand/brand.json` (paleta, tipografia OFL, raios, sombras, movimento, modos glass-dark/glass-light/solid-dark/solid-light/high-contrast) e brand book `brand/book.html`; o usuário revisa
- [X] T081 [US5] Estágio 5: `export_tokens.py` → `frontend/src/theme/tokens.css`; `frontend/src/theme/tokens.ts` passa a ler os mesmos valores; `frontend/src/theme/contrast.test.ts` (portado de `tests/unit/theme/contrast.test.ts`) passa com os valores novos
- [X] T082 [US5] Criar `scripts/check-tokens.mjs` (e no `check.mjs`): falha com cor literal (`#`, `rgb(`, `hsl(`), `font-family` ou `box-shadow` literais em `frontend/src/**/*.css` e em `style={{…}}` fora de `frontend/src/theme/`, e `<svg>` inline fora de `frontend/src/ui/` (FR-169, vindo da T107)
- [X] T083 [US5] Criar componentes próprios `frontend/src/ui/Select.tsx`, `Checkbox.tsx`, `NumberField.tsx` e substituir os 9 controles nativos (configurações, host, encaminhamento)
- [X] T084 [US5] Aplicar a marca em `TitleBar`, barra lateral (grupos recolhíveis com estado em `preferences.collapsedGroups`), abas com estado por cor **e** ícone (FR-163), diálogos com cabeçalho e um único rodapé, estados vazios com ação, SFTP (ícones de arquivo/pasta), encaminhamento, toasts e faixas de erro com ícone
- [X] T085 [US5] Ícone do app: `build/appicon.png` e `build/windows/icon.ico` a partir de `brand/kit/`; `wails.json` (`info.productName`, `companyName` vazio, `copyright`)
- [X] T086 [US5] Implementar `internal/appwin/appearance.go` (registro `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`: `EnableTransparency`, `AppsUseLightTheme`; alto contraste via `SystemParametersInfo`), evento `app:appearance`, preferência `glassEnabled`; fallback sólido sem reiniciar
- [X] T087 [US5] E2E `frontend/tests/e2e/appearance.spec.ts`: vidro desligado → superfícies opacas; terminal 100% opaco por padrão; contraste computado ≥ 4,5:1 nos textos principais dos 5 modos; gera capturas de todas as telas em `test-results/brand/` para revisão
- [X] T107 [P] [US5] Ícones: adicionar `lucide-react`, criar `frontend/src/ui/Icon.tsx` (tamanhos `--icon-sm/md/lg`, `strokeWidth` fixo) e trocar todos os ícones de `frontend/src/ui/icons.tsx` e usos avulsos por Lucide; `scripts/check-tokens.mjs` falha com `<svg>` inline fora de `ui/` (FR-169); licença em `docs/licenses.md`
- [X] T108 [P] [US5] Movimento: tokens `--motion-fast/base/slow` e curvas em `tokens.css` (zerados sob `prefers-reduced-motion`); `frontend/src/ui/motion.ts` com `useExitAnimation` (estado `open|closing` até `animationend`) e `useSlidingIndicator` (FLIP); teste `motion.test.ts` (FR-170)
- [X] T109 [US5] Vidro e estados nos componentes: `.glass-surface`/`.glass-control` e estados repouso/hover/foco/pressionado/selecionado/desabilitado em `Button`, `Menu`, `Popover`, `TextField`, `Select`, `Checkbox`, abas e itens da árvore; indicador deslizante nas abas, na árvore e na navegação das configurações; entrada/saída animada de menus, diálogos e toasts (FR-166, FR-170); depende de T048, T050, T083, T107, T108
- [X] T110 [US5] Barra lateral recolhível: preferência `sidebarCollapsed` em `internal/appwin/preferences.go` e `AppService.UpdatePreferences`; `frontend/src/features/sidebar/Sidebar.tsx` com modo trilho (56 px, ícones hosts/credenciais/SFTP/encaminhamentos/configurações, rótulo em tooltip), botão de recolher e Ctrl+B em `shortcuts.ts`; transição de largura e `fit` sem reconectar (FR-167); depende de T046, T047
- [X] T111 [US5] Página de configurações: `frontend/src/features/settings/SettingsPage.tsx` com navegação por seções (Geral, Aparência, Terminal, Cofre e segurança, Atalhos, Sobre) na área principal, `view` no store de UI, Esc/botão voltar/clique em aba restauram o terminal; mover o conteúdo de `SettingsDialog.tsx` para as seções e remover o diálogo; `TerminalSettings` (T073) vira a seção Terminal; paleta e atalho abrem a página (FR-168)
- [X] T112 [US5] Cor de destaque: preferência `accent` (`system` ou id da marca); `internal/appwin/appearance.go` lê `DWM\AccentColor` e inclui em `app:appearance`; `frontend/src/theme/accent.ts` aplica `--accent*` com ajuste OKLCH até 4,5:1 + `accent.test.ts`; seletor na seção Aparência (FR-171); depende de T086, T111
- [X] T113 [US5] Barra de status: `frontend/src/app/StatusBar.tsx` (host/shell da aba ativa, estado, latência, encaminhamentos ativos, cofre; cada item clicável) e `latencyMs` no `term:state` a partir do RTT do keepalive em `internal/sshx/keepalive.go` (FR-172); depende de T057
- [X] T114 [US5] E2E `frontend/tests/e2e/ui-polish.spec.ts`: Ctrl+B recolhe/expande a lateral mantendo o `sessionId` e `ServerSizeOf` = xterm, estado lembrado após reiniciar; configurações abrem como página e Esc volta à aba; troca de destaque muda `--accent` e mantém contraste ≥ 4,5:1; com `prefers-reduced-motion` emulado as durações computadas são 0; barra de status mostra o host da aba ativa

**Checkpoint**: identidade aplicada

---

## Phase 8: User Story 6 - Arquivos e túneis com a mesma confiabilidade (Priority: P3)

**Goal**: SFTP e encaminhamento local com paridade

**Independent Test**: cenários de US5/US6 da 001 passam; 1 GB idêntico e velocidade ≥ 001

### Tests for User Story 6 ⚠️

- [X] T088 [P] [US6] Portar `tests/unit/sftp/partial.test.ts` para `internal/sftpx/partial_test.go` (`<nome>.svpart-<id8>`, transições, conflitos)
- [X] T089 [P] [US6] Portar `tests/unit/domain/forwardRules.test.ts` para `internal/domain/forwardrules_test.go` ("padrão `127.0.0.1`; outro valor exige `allowNonLoopback: true`", "`bindPort` 1–65535; único entre regras com mesmo `bindAddress`")
- [X] T090 [US6] Portar `tests/integration/sftp.test.ts` para `internal/sftpx/sftp_integration_test.go` (listar, mkdir, renomear, excluir recursivo, ida e volta de 100 MB, cancelamento no meio sem deixar parcial, conflito)
- [X] T091 [US6] Portar `tests/integration/forwarding.test.ts` para `internal/forwarding/forwarding_integration_test.go` (eco via `echo:7000`, porta em uso → `PORT_IN_USE`, desativar libera a porta, queda da conexão → `inactive`, duas regras do mesmo host compartilham uma conexão)

### Implementation for User Story 6

- [X] T092 [US6] Implementar `internal/sftpx/browser.go` e `transfers.go` (`pkg/sftp` com escritas concorrentes, 3 transferências simultâneas, parciais, conflito substituir/pular/cancelar tudo, progresso a 10/s) para passar a T088 e T090
- [X] T093 [US6] Implementar `SftpService` (diálogos nativos para caminhos locais, eventos `sftp:transfer`/`sftp:conflict`)
- [X] T094 [US6] Implementar `internal/domain/forwardrules.go`, `internal/forwarding/manager.go` e `ForwardService` (`ListActive`/`Deactivate` sem cofre) para passar a T089 e T091; T013 passa para todos os 8 serviços
- [X] T095 [US6] Adaptar `frontend/src/features/sftp/*` e `frontend/src/features/forwarding/*` ao adaptador
- [X] T096 [US6] Portar `scripts/bench-sftp.mjs` para `scripts/bench-sftp/main.go` e medir 1 GB (≥ resultado da 001 no mesmo ambiente)
- [X] T097 [US6] Portar `tests/e2e/sftp.spec.ts` e `tests/e2e/forwarding.spec.ts` para `frontend/tests/e2e/`

**Checkpoint**: paridade total

---

## Phase 8b: Ajustes de uso diário (FR-173–FR-177)

**Purpose**: retorno de uso real — atalhos que não roubam teclas do shell, erros explicados,
seção de conexão, shells detectados e opções de terminal que faltavam. Go com o Codex,
frontend com o Claude.

- [X] T115 [P] Go: preferência `shortcuts` (`map[ação]combinação`, `""` = desativado) em `Preferences`/`Settings`, validada (ação conhecida, combinação no formato `Ctrl+Shift+T`, sem duplicata) e devolvida por `GetPreferences`; teste em `internal/appwin`
- [X] T116 Frontend: `frontend/src/app/shortcuts.ts` passa a ler o mapa (padrões Ctrl+Shift de FR-173), seção Atalhos com gravação da combinação, "Desativado", recusa de conflito com o nome da ação e aviso para Ctrl+letra sem Shift; `shortcuts.test.ts`, `TabBar` e textos de ajuda atualizados
- [X] T117 [P] Go: erros da ponte com `op` (serviço.método) e `detail` técnico separado da mensagem, sem segredos; teste em `internal/bridge`
- [X] T118 Frontend: `toastError` com causa por código (`frontend/src/i18n/errors.ts`), código visível e "Copiar detalhes" (código, detalhe, operação, data/hora, versão); teste unitário
- [X] T119 [P] Go: `connectTimeoutSeconds`, `legacyDefault` e `term` (`xterm-256color` | `xterm` | `vt100` | `linux`) nas configurações; `term` vai no pedido de PTY e `legacyDefault` vale para hosts criados sem o campo; testes
- [X] T120 Frontend: seção "Conexão (SSH)" nas configurações (keepalive e tempo limite saem de "Cofre e segurança", padrão de algoritmos legados, `$TERM` por seleção); formulário de host novo usa `legacyDefault`
- [X] T121 [P] Go: detecção de shells — `pwsh` (PATH e `%ProgramFiles%\PowerShell\*`), Git Bash (`%ProgramFiles%\Git\bin\bash.exe` e registro), distros WSL (`wsl.exe -l -q`, saída UTF-16) — em `LocalShell` com id estável; testes com leitores falsos
- [X] T122 Frontend: shell padrão em Geral e menu de nova aba com os shells detectados, cada um com ícone próprio
- [X] T123 [P] Go: preferências `copyOnSelect`, `rightClickPaste`, `bell` (`off` | `sound` | `visual`), `boldBright` e `autoReconnect`; testes
- [X] T124 Frontend: opções de FR-177 na seção Terminal (por seleção) e comportamento no `TerminalView` (copiar ao selecionar, colar no botão direito, sino, `drawBoldTextInBrightColors`, reconexão automática com recuo)
- [X] T125 E2E `frontend/tests/e2e/daily-use.spec.ts`: Ctrl+W chega ao shell e a aba fica; Ctrl+Shift+W fecha; remapear e desativar persiste após reiniciar; erro mostra código e copia detalhes; `$TERM` escolhido aparece no `echo $TERM` do servidor de teste

## Phase 8c: Logo do sistema no host (FR-178)

**Purpose**: pedido do usuário em 2026-10-25 — o avatar do host mostra o logo do sistema
(Ubuntu, Debian, MikroTik, Cisco…), como no Termius. Go com o Codex, frontend com o Claude.

- [ ] T126 [P] Go: `detectedOs` e `osIcon` no Host (data-model; ids da lista fechada, validação, duplicar copia, cofre antigo lê `null`/`"auto"`); `internal/sshx` classifica a versão SSH do servidor e, só para OpenSSH sem distribuição na versão, lê `/etc/os-release` num canal `exec` à parte (tempo limite 3 s, falha silenciosa, nunca na aba do usuário, nunca em equipamento de rede); ao mudar, grava no cofre e emite `hosts:changed`; testes de classificação com versões reais e do `os-release`
- [ ] T127 Frontend: logos de sistema embutidos (`frontend/src/ui/osIcons.tsx`, Simple Icons CC0, monocromáticos no tom do tema), `Avatar` mostra o logo de `osIcon`/`detectedOs` e cai nas iniciais; seletor "Ícone" no formulário do host; recarrega a lista em `hosts:changed`; licença em `docs/licenses.md`
- [ ] T128 E2E: conectar ao servidor de teste muda o avatar para o logo detectado; escolher outro ícone no formulário persiste após reiniciar; "Automático" volta ao detectado

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T098 E2E `frontend/tests/e2e/secrets-scan.spec.ts` (SC-108): fluxo completo e varredura recursiva do diretório de dados (incluindo `.db`, `-wal`, pasta do WebView2) por senha mestra, senhas, passphrase e trechos da chave em UTF-8, UTF-16LE e base64 → 0 ocorrências; nenhum `*.dmp`, `*.log` nem `Crashpad/reports` com arquivos
- [X] T099 [P] E2E `frontend/tests/e2e/performance.spec.ts` (SC-112): 20 abas, uma com `yes` contínuo, troca de aba < 100 ms; busca em 1.000 hosts < 200 ms
- [X] T100 [P] Criar `scripts/measure.ps1` (SC-109): tamanho do instalador e soma do *private working set* do app + processos WebView2 após 2 min ocioso com 1 aba; registrar em `specs/002-wails-migration/validation.md`; se > 150 MB, aplicar mitigações de research R19 e registrar números
- [X] T101 Revisão de segurança em `docs/security-review-002.md` conferindo cada linha da tabela do plan.md contra o código e corrigindo achados
- [X] T102 [P] `docs/licenses.md`: dependências Go (`go-licenses report`), npm do frontend, fontes OFL e origem dos presets de cor
- [X] T103 [P] Atualizar `README.md`: requisitos (Go, Wails, NSIS, Docker), `wails dev`, `wails build -nsis`, testes, migração do cofre da versão anterior
- [ ] T104 ⏸ usuário — Validar `quickstart.md` de ponta a ponta (cenários manuais, incluindo SC-102 nos equipamentos reais) e registrar em `specs/002-wails-migration/validation.md`
- [ ] T105 Gerar instalador com `wails build -clean -nsis -trimpath -ldflags "-s -w"` e `scripts/check-release.mjs`; validar instalação limpa e desinstalação no Windows 11 e fallback sólido no Windows 10
- [X] T106 Remover a base Electron após a paridade validada: `src/main`, `src/preload`, `electron.vite.config.ts`, `electron-builder.yml`, configs TS da raiz, testes antigos já portados (`tests/unit`, `tests/integration`, `tests/e2e`), dependências do `package.json` da raiz (que fica só com scripts de teste/fixtures); tudo verde em `scripts/check.mjs`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (1)** → **Foundational (2)** → histórias.
- **US1 (3)** é base de todas as outras (cofre, sessões e ponte).
- **US2 (4)** depende de US1 (sessões para medir layout); **US3 (5)** depende de US1.
- **US4 (6)** depende de US1; o padrão de cor da marca (T068) usa T081 se já existir, senão um padrão provisório substituído depois.
- **US5 (7)**: T077–T080 dependem só do usuário e podem começar a qualquer momento em paralelo; T081–T087 dependem de T080 e de US2 (componentes de layout).
- **US6 (8)** depende de US1.
- **Polish (9)** depende de todas; T106 é a última.

### Within Each User Story

- Testes antes e falhando → domínio → serviços Go → ponte → frontend → E2E.

### Parallel Opportunities

- Setup: T006–T008 e T010 em paralelo.
- US1: T019–T024 e T026 em paralelo; T036 em paralelo com T027–T035.
- US2: T043–T045 em paralelo.
- US3: T052, T053 e T055 em paralelo.
- US4: T065–T067 em paralelo; T070 e T072 em paralelo com T068–T069.
- US5: os estágios ⏸ do `brand-kit` correm em paralelo com US2–US4 enquanto o usuário escolhe.
- US6: T088 e T089 em paralelo; US6 inteira pode correr em paralelo com US3/US4 depois de US1.

## Parallel Example: User Story 1

```text
T019 internal/vault/crypto_test.go
T020 internal/vault/store_test.go
T021 internal/vault/migrate001/migrate001_test.go
T022 internal/domain/*_test.go
T023 internal/session/aggregator_test.go
T024 internal/session/manager_test.go
T026 internal/localpty/pty_integration_test.go
```

## Implementation Strategy

### MVP First (User Story 1)

1. Setup + Foundational.
2. US1 → validar: migração do cofre real, prompt único, 100 ciclos sem vazamento.
3. Parar e validar com o usuário antes de seguir.

### Incremental Delivery

1. US1 (núcleo) → US2 (layout) → US3 (NOC): app usável no dia a dia do NOC.
2. US6 (paridade SFTP/túneis) → a base Electron pode deixar de ser usada.
3. US4 (personalização) e US5 (marca), com os checkpoints do usuário.
4. Polish e remoção da base Electron.

## Notes

- [P] = arquivos diferentes, sem dependência pendente.
- Commits pequenos em Conventional Commits por tarefa ou grupo lógico.
- Tarefas ⏸ param para o usuário; as demais continuam.
- Nenhuma tarefa concluída com teste falhando.
