# Research: Migração do ShellVault para Go + Wails (Fase 1)

**Feature**: `002-wails-migration` | **Date**: 2026-10-23

Cada item: **Decisão**, **Justificativa**, **Alternativas**. Versões e licenças MUST ser
reconfirmadas na tarefa que adota a dependência (Princípio I).

## R1. Framework desktop: Wails v2.17

- **Decisão**: Wails v2.17 (estável). Janela sem moldura (`Frameless: true`), WebView2 com
  `WebviewIsTransparent` + `WindowIsTranslucent` e `BackdropType` Mica (fallback Acrylic e,
  sem suporte, cor sólida), `SingleInstanceLock` com `OnSecondInstanceLaunch` trazendo a
  janela para frente.
- **Justificativa**: v3 ainda é beta (v3.0.0-beta.32, sem data de GA em out/2026); v2 é a linha
  estável que recebe correções. Wails usa o WebView2 do sistema: binário pequeno e nenhum
  Chromium empacotado.
- **Alternativas**: Wails v3 beta (API de janela mais rica, regiões não-cliente nativas para
  snap layouts) — rejeitado por estabilidade; Tauri (Rust, fora da decisão do usuário);
  manter Electron (é o que se está substituindo).
- **Limite registrado**: em v2, botões de janela desenhados em HTML não disparam o menu de
  snap layouts ao passar o mouse (requer regiões não-cliente nativas do v3). Arrasto até a
  borda, duplo clique e Win+setas funcionam. Spec FR-133 ajustada.

## R2. Linguagem e toolchain

- **Decisão**: Go 1.26.x (linha estável confirmada; usar 1.27 se já estiver estável na
  instalação), `go.mod` com `go 1.26`, `CGO_ENABLED=0` no build de release. Frontend:
  TypeScript 6 `strict`, Node 24 LTS só como ferramenta de build.
- **Justificativa**: build sem CGO simplifica o pipeline Windows e é exigido por
  `modernc.org/sqlite`. TypeScript `strict` mantém a regra de tipagem estática da
  constituição no frontend.
- **Alternativas**: CGO + `mattn/go-sqlite3` (exige gcc no Windows) — rejeitado.
- **Nota de teste**: `go test -race` exige CGO e gcc; roda em job Linux para os pacotes
  independentes de plataforma e localmente quando houver gcc. Vazamento de goroutines é
  verificado sem `-race`, com `go.uber.org/goleak` (MIT) e `runtime.NumGoroutine`.
- **Versões instaladas (T001, 2026-10-23)**: Go 1.27.3 (zip oficial em
  `%LOCALAPPDATA%\Programs\go`, SHA-256 conferido com go.dev), Wails CLI v2.17.0, Node
  24.22.1, npm 11.19.1, WebView2 155.0.4283.62. `wails doctor` sem erros. NSIS fica para a
  T105 (instalador).

## R3. Armazenamento: SQLite com conteúdo cifrado por linha

- **Decisão**: `modernc.org/sqlite` (BSD-3, sem CGO), arquivo `%APPDATA%\ShellVault\shellvault.db`,
  modo WAL, `synchronous=FULL`, `foreign_keys=ON`, `secure_delete=ON`. Cada entidade do cofre
  vira uma linha com colunas em claro só para estrutura (ids, `parent_id`, `group_id`,
  `sort_order`, timestamps) e uma coluna `sealed` com o JSON da entidade cifrado pela DEK.
  Preferências não sensíveis (tema, perfis de terminal, esquemas de cores, estado da janela)
  ficam em claro, porque são necessárias antes do desbloqueio.
- **Justificativa**: FR-101 mantém FR-002 da 001 (hosts e credenciais cifrados — endereço,
  usuário e rótulo de um host de NOC são sensíveis). Cifrar por linha permite gravações
  pequenas e atômicas (transação SQLite) em vez de reescrever o cofre inteiro, e mantém a
  busca em memória após o desbloqueio, como na 001. `secure_delete` sobrescreve páginas
  liberadas.
- **Alternativas**: SQLite com hosts em claro e só segredos cifrados (regride FR-002 —
  rejeitado); SQLCipher (exige CGO — rejeitado); continuar com arquivo único `.svlt`
  (perde a escolha do usuário por SQLite e as gravações incrementais).

## R4. Criptografia compatível com o cofre da 001

- **Decisão**: `golang.org/x/crypto/argon2.IDKey` e `golang.org/x/crypto/chacha20poly1305.NewX`
  (BSD-3). Mesmo esquema de envelope da 001: KEK = Argon2id(senha, salt), DEK aleatória de
  32 bytes embrulhada pela KEK; nonce aleatório de 24 bytes por cifragem. Custo padrão igual ao
  `MODERATE` do libsodium (3 passes, 256 MiB, paralelismo 1). AD de cada linha = `tabela ‖ id`,
  impedindo trocar linhas cifradas entre si.
- **Compatibilidade**: o `crypto_pwhash` do libsodium com Argon2id13 equivale a
  `argon2.IDKey(senha, salt, opsLimit, memLimit/1024, 1, 32)`; `crypto_aead_xchacha20poly1305_ietf`
  equivale a `chacha20poly1305.NewX`. O AD do embrulho da DEK na 001 é o JSON
  `{"v":1,"kdf":{"alg":…,"opsLimit":…,"memLimit":…,"salt":…},"createdAt":…}` serializado pelo
  `JSON.stringify` — o Go MUST reproduzir esses bytes exatamente (ordem de chaves e números
  inteiros), validado por teste com cofre gerado pela 001.
- **Memória**: chaves em `[]byte` zeradas no bloqueio; strings Go não são zeráveis — mesmo
  risco aceito e documentado da 001.
- **Alternativas**: libsodium via CGO (rejeitado por CGO); AES-256-GCM (permitido pela
  constituição, mas quebraria a compatibilidade direta com a 001).

## R5. Migração do cofre da 001 (FR-102, FR-103)

- **Decisão**: com o cofre ainda inexistente no banco e `vault.svlt` presente (estado
  `migratable`), a tela de desbloqueio pede a senha mestra, decifra o `.svlt` (formato v1,
  `schemaVersion` 1), grava tudo no `shellvault.db` numa única transação, lê de volta e confere
  o conteúdo inteiro, e só então renomeia o original para `vault.svlt.migrated` (mantido; o
  usuário pode apagar depois). Falha antes do commit desfaz a transação; falha na conferência
  apaga o cofre recém-criado. Nos dois casos o `.svlt` fica intacto e o estado volta a
  `migratable`. `settings.json` da 001 é importado para as preferências.
  *(Revisto na T031: o banco fica sempre aberto por causa das preferências, então a transação
  substitui o arquivo `.tmp` previsto antes.)*
- **Justificativa**: Princípio III (caminho de migração documentado) e SC-114. A KEK é
  derivada de novo com sal novo; a DEK é gerada nova (não há motivo para carregar a antiga).
- **Teste**: fixture `tests/fixtures/vault-001/` gerada pelo código Electron com `TEST_KDF`
  (hosts, grupos aninhados, as duas formas de credencial, host keys, regras) — o teste Go
  migra e compara campo a campo.

## R6. SSH: `golang.org/x/crypto/ssh`

- **Decisão**: cliente `x/crypto/ssh` (BSD-3); host key por `HostKeyCallback` próprio
  consultando a tabela `known_hosts` do cofre (fingerprint `ssh.FingerprintSHA256`); o pacote
  `knownhosts` é usado só para normalizar `host:port` (`knownhosts.Normalize`), já que o
  armazenamento fica no cofre e não em `~/.ssh/known_hosts`. Autenticação: senha,
  `PublicKeys` (chave com passphrase via `ssh.ParseRawPrivateKeyWithPassphrase`),
  `KeyboardInteractive` (prompts encaminhados à interface como na 001) e agente.
- **Keepalive (FR-120)**: goroutine por conexão envia `keepalive@openssh.com` com `wantReply`
  a cada 30 s; 3 falhas seguidas (ou erro de escrita) fecham a conexão e a aba vai para
  `disconnected`.
- **Algoritmos legados (FR-121)**: com a opção desligada, as listas padrão da biblioteca.
  Ligada: `KeyExchanges` = padrão + `diffie-hellman-group14-sha1`, `diffie-hellman-group1-sha1`;
  `Ciphers` = padrão + `aes128-cbc`, `3des-cbc`; `HostKeyAlgorithms` = padrão + `ssh-rsa`;
  chaves RSA do usuário assinam também com `ssh-rsa` (SHA-1) via
  `ssh.NewSignerWithAlgorithms`. Os modernos ficam primeiro, então são preferidos.
  `aes256-cbc` não existe na biblioteca (limite registrado na spec).
- **Erro sem algoritmo em comum (FR-122)**: a mensagem `no common algorithm for …` da
  biblioteca é mapeada para `NO_COMMON_ALGORITHM`, com texto pt-BR sugerindo a opção.
- **Alternativas**: `golang.org/x/crypto/ssh` é a escolha da spec e a referência em Go;
  nenhuma alternativa avaliada.

## R7. Agente SSH (FR-124)

- **Decisão**: `golang.org/x/crypto/ssh/agent` sobre o named pipe do agente do OpenSSH para
  Windows (`\\.\pipe\openssh-ssh-agent`), aberto com `github.com/Microsoft/go-winio` (MIT);
  em Linux/macOS, `SSH_AUTH_SOCK` (não testado nesta fase). Método de autenticação do host
  `agent`, sem credencial no cofre.
- **Alternativas**: Pageant (protocolo diferente) — fora; pode entrar depois sem mudar o
  modelo.

## R8. Shell local: interface PTY

- **Decisão**: interface `pty.PTY` (Start, Read, Write, Resize, Wait, Close) implementada com
  `github.com/aymanbagabas/go-pty` (MIT), que cobre ConPTY no Windows e PTY Unix numa API só.
  Shells por caminho absoluto em `%SystemRoot%\System32` (regra da 001).
- **Justificativa**: uma única dependência para as duas plataformas, usada no ecossistema
  charmbracelet; atende "ConPTY no Windows e creack/pty atrás de uma interface" da SPEC.
- **Alternativas**: `UserExistsError/conpty` + `creack/pty` (duas dependências e duas
  implementações); `charmbracelet/x/conpty` (só Windows).

## R9. SFTP: `github.com/pkg/sftp`

- **Decisão**: `pkg/sftp` (BSD-2) sobre o mesmo `ssh.Client` da conexão SFTP, com
  `UseConcurrentWrites(true)`, `MaxConcurrentRequestsPerFile(64)` e cópia via
  `File.ReadFrom`/`WriteTo` (pipelines internos). Regras de transferência da 001 mantidas:
  arquivo parcial `<nome>.svpart-<id8>`, renomeia ao concluir, fila com 3 transferências
  simultâneas, conflito com substituir/pular/renomear/aplicar a todos.
- **Meta**: SC-006 da 001 (≥ 80% do `sftp.exe`) e, pela spec, não ficar abaixo da 001;
  o benchmark `scripts/bench-sftp` é portado.

## R10. Ponte interface ↔ Go (Princípio II)

- **Decisão**: bindings do Wails expõem só structs de serviço (`VaultService`, `HostService`,
  `CredentialService`, `SessionService`, `SftpService`, `ForwardService`, `ProfileService`,
  `AppService`), cada método validando argumentos no Go e devolvendo `Result{ok, value |
  error{code, message}}` como na 001. Eventos Go → interface por `runtime.EventsEmit` com
  nomes fixos. Nenhum método devolve segredo. CSP no `index.html`:
  `default-src 'self'; script-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline';
  img-src 'self' data:; font-src 'self' data:`.
- **Saída do terminal (FR-112, FR-115)**: uma goroutine de leitura por sessão; um agregador
  junta bytes por até 8 ms ou 32 KiB e emite `term:data` com `{id, data}`. Os bytes passam por
  um decodificador UTF-8 incremental (bytes de um caractere incompleto ficam para o próximo
  lote), então o evento carrega texto e não base64. Escrita da interface por
  `SessionService.Write(id, text)`.
- **Assinatura única**: a interface registra **um** ouvinte global de `term:data` no início do
  app e despacha por `id` num mapa (padrão `dataBus` da 001); a sessão não registra ouvintes.
- **Alternativas**: um evento por sessão (`term:data:<id>`, como sugeria a SPEC) — rejeitado
  porque cria/remove ouvintes a cada aba, origem provável do prompt duplicado; WebSocket local
  (porta aberta na máquina) — rejeitado.

## R11. Ciclo de vida das sessões (FR-114, SC-105)

- **Decisão**: `session.Manager` com `map[id]*Session` sob mutex; cada sessão tem
  `context.WithCancel`; fechar cancela o contexto, fecha canal, cliente SSH/PTY e espera as
  goroutines (`sync.WaitGroup`) antes de emitir `term:closed`. Teste de 100 aberturas e
  fechamentos com `goleak.VerifyNone` e comparação de `runtime.NumGoroutine`.
- **Processo (FR-110, FR-111)**: nenhum `exec` aponta para `os.Executable()`; teste estático
  (grep no código) e teste E2E contando processos e janelas do app após abrir shells e
  transferências.

## R12. Layout e redimensionamento (FR-130, FR-131)

- **Decisão**: raiz `height: 100vh` com grid `sidebar | main` e `min-height: 0`/`min-width: 0`
  em todos os filhos flex/grid; `ResizeObserver` no contêiner do terminal → `fit()` com debounce
  de 50 ms → `SessionService.Resize` só com `cols > 0 && rows > 0` e só se mudou. O primeiro
  resize sai depois do primeiro `fit` válido (FR-113). `devicePixelRatio` monitorado via
  `matchMedia('(resolution: …dppx)')` para refazer o fit ao trocar de monitor.
- **Teste**: E2E maximiza/restaura/redimensiona 20 vezes (métodos de teste do Wails runtime em
  build `e2e`) em 100/125/150% e compara captura com a verificação de "nenhum pixel da cor de
  fundo da janela dentro da área do terminal" + tamanho recebido pelo servidor.

## R13. Menus ancorados (FR-132)

- **Decisão**: `@floating-ui/react-dom` (MIT, ~3 KiB) para posicionar menus e popovers com
  `flip`, `shift` e `size`, renderizados em portal no `body` com `z-index` acima do terminal.
- **Justificativa**: o bug da 001 veio de CSS global (`position: relative` em `.glass-edge`)
  sobrescrevendo `position: fixed`; o posicionamento por colisão manual é fácil de errar.
- **Alternativas**: cálculo próprio (já falhou); Radix/Headless UI (mais peso e estilo
  próprio).

## R14. Terminal (xterm.js)

- **Decisão**: `@xterm/xterm` 6 com addons `fit`, `webgl` (fallback para o renderizador DOM
  no `onContextLoss`, já que o canvas saiu do core no xterm 6), `unicode11`, `search` e
  `web-links` (abre via `runtime.BrowserOpenURL`, só com Ctrl+clique). `allowTransparency`
  ligado apenas quando a opacidade do perfil < 100%.
- **Trade-off documentado**: transparência com WebGL custa desempenho e pode borrar
  sub-pixel; o padrão é 100% opaco (FR-150), com aviso ao reduzir.

## R15. Personalização: fontes, esquemas e perfis

- **Fontes do sistema**: Go varre `%WINDIR%\Fonts` e `%LOCALAPPDATA%\Microsoft\Windows\Fonts`,
  lê cada arquivo com `golang.org/x/image/font/sfnt` (BSD-3) e lista as famílias com
  `isFixedPitch` (tabela `post`) ou largura uniforme. Resultado em cache por sessão do app.
- **Fontes embutidas**: JetBrains Mono (OFL-1.1, com ligaduras) e Cascadia Mono/Code
  (OFL-1.1), em `woff2` no bundle do frontend.
- **Importação**: Windows Terminal — objeto de esquema JSON (`name`, `foreground`,
  `background`, `cursorColor`, `selectionBackground`, `black`…`brightWhite`), aceitando também
  um `settings.json` inteiro (lista `schemes`, o usuário escolhe). iTerm2 — plist XML
  (`Ansi 0 Color`…`Ansi 15 Color`, `Foreground/Background/Cursor/Selection Color`, componentes
  float 0–1, espaço sRGB; P3 convertido para sRGB). Parser com `encoding/xml` da biblioteca
  padrão. Cores ausentes completadas pelo preset padrão (spec, Edge Cases).
- **Presets**: valores das paletas oficiais publicadas sob licença MIT de cada projeto
  (Dracula, Nord, Solarized, One Dark, Tokyo Night, Catppuccin) — só os valores de cor,
  registrados com a origem em `docs/licenses.md`.
- **Aplicação ao vivo**: perfil efetivo = global ⊕ override do host; mudança emite
  `profiles:changed`; cada aba aplica `term.options.* = …` e `fit()`.

## R16. Aparência e identidade

- **Decisão**: tokens gerados pelo skill `brand-kit` (`brand.json` → `export_tokens.py` →
  `tokens.css`) com modos glass-dark, glass-light, solid-dark, solid-light e high-contrast; o
  frontend consome só variáveis CSS (lint de estilo proíbe cores literais fora dos tokens).
  Detecção de suporte e preferências do Windows (transparência, alto contraste, tema) no Go
  (registro `HKCU\…\Themes\Personalize`), emitindo `app:appearance`.
- **Ícone**: `make_kit.py` gera o `.ico` e o PNG usados em `build/windows/icon.ico` e
  `build/appicon.png` do Wails.

## R16a. Ícones, movimento, destaque e barra de status (FR-166–FR-172)

- **Ícones**: `lucide-react` (ISC, ícones em traço, *tree-shaking* por ícone). Um wrapper
  `frontend/src/ui/Icon.tsx` fixa `strokeWidth` e os tamanhos dos tokens (`--icon-sm` 16,
  `--icon-md` 20, `--icon-lg` 24); `frontend/src/ui/icons.tsx` atual passa a reexportar só
  ícones do Lucide. Alternativas rejeitadas: Phosphor (pacote maior), ícones desenhados à mão
  (inconsistência que motivou o pedido).
- **Movimento**: só CSS (transições e `@keyframes`) com durações e curvas dos tokens
  (`--motion-fast` 120 ms, `--motion-base` 180 ms, `--motion-slow` 250 ms); entrada/saída de
  menus e diálogos com estado `data-state="open|closing"` e `animationend`; indicador de seleção
  deslizante por FLIP (mede o item, anima `transform`). `@media (prefers-reduced-motion: reduce)`
  zera os tokens de duração. Rejeitado `framer-motion`/`motion` (~40 KB) por não ser necessário.
- **Vidro em componentes**: classes utilitárias `.glass-surface`, `.glass-control` sobre
  `backdrop-filter` com tokens de opacidade/desfoque; estados por `:hover`, `:focus-visible`,
  `:active`, `[aria-selected=true]`, `:disabled`. Sem vidro (FR-162), os mesmos tokens viram
  superfícies opacas.
- **Cor de destaque**: preferência `accent` (`"system"` ou id de uma cor da marca). Com
  `"system"`, o Go lê `HKCU\Software\Microsoft\Windows\DWM\AccentColor` (ABGR) e emite em
  `app:appearance`; o frontend define `--accent*` e ajusta luminosidade em OKLCH até 4,5:1 contra
  a superfície do tema.
- **Barra lateral**: preferência `sidebarCollapsed` (bool); trilho de 56 px; transição de
  largura no grid com o `ResizeObserver` do terminal fazendo o `fit` ao fim (T047).
- **Configurações**: rota interna `view: "terminal" | "settings"` no store de UI, sem roteador;
  seção ativa em `preferences.settingsSection` não é necessária (abre sempre em Geral).
- **Latência**: RTT do último `keepalive@openssh.com` (T057) exposto em `term:state`
  (`latencyMs`); sem keepalive, a barra de status mostra "—".

## R17. Offline e ausência de logs (FR-027, FR-030)

- **Decisão**: frontend sem nenhuma chamada de rede (CSP `connect-src 'self'`); o Wails só
  serve assets embutidos. Logger do Wails com nível `ERROR` para stdout (nada em arquivo).
  WebView2 recebe `--disable-breakpad --disable-crash-reporter` via patch local do
  `go-webview2` (`edge.ExtraBrowserArgs`; o loader zera `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`,
  ver `third_party/go-webview2/PATCH.md`) antes de `wails.Run`, e `WebviewUserDataPath` dentro
  do diretório de dados; teste varre o diretório por `Crashpad/reports` e `*.dmp`.

## R18. Testes

- **Unitários Go**: `go test ./internal/...` (cofre, cripto, migração, domínio, importadores de
  esquema, algoritmos, agregador de saída, decodificador UTF-8).
- **Integração**: mesmo OpenSSH em Docker da 001 (`tests/fixtures/sshd`) + um contêiner novo
  `sshd-legacy` que aceita **só** algoritmos legados (OpenSSH com `KexAlgorithms`, `Ciphers` e
  `HostKeyAlgorithms` restritos) + um contêiner `idle-killer` (proxy TCP que derruba conexões
  ociosas após 60 s) para SC-110.
- **Frontend**: Vitest para lógica pura (busca, árvore, perfis, paleta, colagem).
- **E2E**: build com tag `e2e` habilita `--remote-debugging-port` no WebView2 e uma
  `TestService` (maximizar, restaurar, tamanho, contagem de goroutines); Playwright conecta por
  `chromium.connectOverCDP`. Build de release MUST NOT conter a tag (verificado no CI e por teste
  que procura a string da porta no binário).
- **Equipamentos reais** (SC-102): roteiro manual em `quickstart.md`, com acesso do usuário.

## R19. Tamanho e memória (SC-109)

- **Decisão**: `wails build -ldflags "-s -w" -trimpath`, instalador NSIS do Wails
  (`-nsis`, exige NSIS instalado). Estimativa: binário 12–18 MB, instalador < 15 MB.
- **Risco**: memória do WebView2 (processos `msedgewebview2.exe`) pode passar de 150 MB com
  WebGL ativo. Medição pela soma do *private working set* da árvore de processos do app; se
  estourar, mitigação em ordem: desligar GPU raster extra, limitar scrollback padrão, avaliar
  renderizador DOM. Registrado no plano como risco.

## R20. Constituição

- **Decisão**: emenda PATCH 1.0.1 antes da implementação: os exemplos de bibliotecas do
  Princípio V (`ssh2`, `node-pty`) e de linguagem ("TypeScript strict") passam a citar Go +
  TypeScript strict e as bibliotecas desta pesquisa; nenhum princípio muda de sentido. A regra
  "processo de interface isolado, ponte mínima e validada" é atendida pelo WebView2 sem acesso
  ao sistema + bindings validados (R10).
