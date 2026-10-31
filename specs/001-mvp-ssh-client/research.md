# Research: MVP do cliente SSH ShellVault

**Feature**: `001-mvp-ssh-client` | **Date**: 2026-10-23

Versões consultadas no registro npm em 2026-10-23. Todas as licenças foram conferidas
(Princípio I); nenhuma é copyleft.

## R1. Framework desktop

- **Decision**: Electron 45 (MIT), com `contextIsolation: true`, `sandbox: true`,
  `nodeIntegration: false`, preload mínimo via `contextBridge`. Build com electron-vite 6
  (MIT) e empacotamento com electron-builder 26 (MIT, instalador NSIS para Windows x64).
- **Rationale**: a constituição pede processos separados (principal, interface sem
  privilégios, ponte tipada) e cita `ssh2`/`node-pty`, que são bibliotecas Node. Electron dá
  isso direto, roda em Windows 10/11 e porta para macOS/Linux depois. Tem `backgroundMaterial`
  nativo para o vidro (R8).
- **Alternatives considered**: Tauri 2 (mais leve, mas exige SSH em Rust — `russh` — e
  terminal local próprio, perdendo `ssh2`/`node-pty` citados no Princípio V); WinUI 3/.NET
  (Windows-only, contraria a portabilidade desejada).

## R2. Linguagem e ferramentas

- **Decision**: TypeScript 6.0.x em modo `strict` (+ `noUncheckedIndexedAccess`), ESLint 9 +
  typescript-eslint, Prettier. Node do Electron 45 em runtime; Node 24 LTS para ferramentas.
- **Rationale**: tipagem estática obrigatória. TypeScript 7 (port nativo) já existe, mas o
  typescript-eslint atual aceita `<6.1.0`; ficar em 6.0 evita quebrar o lint.
- **Alternatives considered**: TypeScript 7.0 (migrar quando o typescript-eslint suportar).

## R3. Interface

- **Decision**: React 19 (MIT) + Zustand 5 (MIT) para estado da interface; CSS próprio
  (CSS Modules) sem biblioteca de componentes; ícones SVG desenhados no projeto.
- **Rationale**: FR-031 proíbe componentes/ícones de terceiros na aparência. Zustand evita
  boilerplate de contexts para estado compartilhado entre abas, barra lateral e diálogos
  (justificado em Complexity Tracking).
- **Alternatives considered**: Context puro do React (re-render em cascata com 20 abas);
  Redux Toolkit (mais cerimônia sem ganho no MVP).

## R4. Emulador de terminal

- **Decision**: `@xterm/xterm` 6 (MIT) + `@xterm/addon-fit` (redimensionamento),
  `@xterm/addon-webgl` (renderização rápida, fallback para DOM se WebGL falhar),
  `@xterm/addon-unicode11` (largura correta de caracteres Unicode). `scrollback: 10000`.
- **WebGL só na aba visível**: o Chromium limita os contextos WebGL ativos por página
  (~16); com 20 abas os mais antigos seriam perdidos. O addon WebGL é carregado ao ativar a
  aba e descartado ao ocultá-la (aba oculta usa o renderer DOM e não desenha). `fit`/resize
  são ignorados enquanto a aba está com `display:none` e reaplicados ao reexibir.
- **Rationale**: cobre FR-015 (256 cores/true color, Unicode, rolagem, copiar/colar) e
  SC-007 (WebGL mantém troca de aba < 100 ms; abas ocultas não renderizam).
- **Alternatives considered**: implementar emulador próprio (fora de questão, Princípio V).

## R5. SSH, SFTP e port forwarding

- **Decision**: `ssh2` 1.17 (MIT), apenas no processo principal.
  - Host key: opção `hostVerifier(key, cb)` assíncrona; impressão digital
    `SHA256:<base64 sem padding>` do blob da chave; comparação com `KnownHostKey` do cofre.
  - Autenticação: `password`, `privateKey` + `passphrase`, `tryKeyboard: true` com evento
    `keyboard-interactive` repassado à interface como prompts.
  - `readyTimeout: 15000` (edge case de 15 s); `keepaliveInterval: 15000` para detectar queda.
  - Chaves: `utils.parseKey` valida OpenSSH/PEM (Ed25519, ECDSA, RSA) e passphrase na
    importação (FR-008); `utils.generateKeyPairSync('ed25519')` para FR-009.
  - SFTP: `fastPut`/`fastGet` (leitura/escrita paralela, necessária para SC-006 ≥ 80% do
    `scp`/`sftp` CLI) com callback `step` para progresso. Cada transferência usa canal SFTP
    próprio: cancelar = fechar o canal.
  - Port forwarding: `net.createServer` no endereço/porta locais; cada socket aceito →
    `client.forwardOut(...)` e `pipe` bidirecional.
- **Rationale**: biblioteca consolidada, pura JS (sem rebuild nativo), cobre todos os FRs.
- **Alternatives considered**: invocar `ssh.exe` do Windows (sem controle de host key/UI de
  prompts, parsing frágil); `node-libssh` (binding nativo menos mantido).

## R6. Conexões por funcionalidade

- **Decision**: uma conexão `ssh2.Client` por aba de terminal e por aba SFTP; uma conexão
  compartilhada por host para todas as suas regras de encaminhamento ativas. A conexão de
  encaminhamento fecha quando a última regra é desativada.
- **Rationale**: isola falhas (queda de uma aba não derruba outra), simplifica estados.
  Multiplexar tudo em uma conexão por host seria otimização prematura.
- **Alternatives considered**: pool por host (mais estados compartilhados e casos de erro).

## R7. Terminal local

- **Decision**: `node-pty` 1.1 (MIT), ConPTY no Windows, rodando no processo principal.
  Shells: `powershell.exe` (Windows PowerShell 5.1) e `cmd.exe`, resolvidos em
  `%SystemRoot%\System32`, `cwd = os.homedir()`. Sem rebuild manual: usar prebuild/N-API do
  pacote; se o Electron 45 exigir, `@electron/rebuild` no `postinstall`.
- **Rationale**: FR-017; caminho absoluto evita executar binário homônimo do PATH.
- **Alternatives considered**: `child_process.spawn` sem PTY (sem cores, sem resize).

## R8. Vidro translúcido e fallback

- **Decision**: `BrowserWindow` com `backgroundMaterial: 'acrylic'` (Windows 11 22H2+) e
  `backgroundColor: '#00000000'`; a moldura usa fundos `rgba` sobre o acrílico + borda de 1 px
  com gradiente claro (brilho). Terminal e SFTP com fundo opaco (FR-032).
  Fallback sólido (`backgroundMaterial: 'none'` + tokens CSS opacos) quando:
  `nativeTheme.prefersReducedTransparency` (efeitos de transparência desligados),
  `nativeTheme.shouldUseHighContrastColors` (alto contraste) ou build do Windows < 22621.
  Mudanças são observadas pelo evento `nativeTheme.on('updated')` e enviadas à interface.
- **Contraste (FR-034)**: a camada de vidro sempre tem tinta mínima (opacidade ≥ 0,72 escuro /
  ≥ 0,78 claro) calibrada para que texto primário e secundário passem 4,5:1 contra fundo
  totalmente branco e totalmente preto atrás da janela; verificado por teste de tokens (R11).
- **Rationale**: material do próprio SO é desfocado pelo compositor (custo zero de CPU,
  SC-009); visual (cores, bordas, raio) é criação própria.
- **Alternatives considered**: `backdrop-filter` em janela transparente (só desfoca o conteúdo
  da própria página, não o que está atrás da janela); Mica (não desfoca o conteúdo de trás).

## R9. Cofre criptografado

- **Decision**: `libsodium-wrappers-sumo` 0.8 (ISC), no processo principal.
  - KDF: Argon2id (`crypto_pwhash`, `OPSLIMIT_MODERATE`, `MEMLIMIT_MODERATE` = 256 MiB),
    salt aleatório de 16 bytes. Parâmetros gravados no cabeçalho para poder mudá-los depois.
  - Envelope: chave de dados (DEK) aleatória de 32 bytes cifra o conteúdo; a chave derivada
    da senha (KEK) cifra a DEK. Ambos com XChaCha20-Poly1305 IETF (nonce de 24 bytes novo a
    cada gravação). Trocar a senha (FR-004) só recifra a DEK.
  - Cabeçalho (magic, versão de formato, parâmetros KDF) é passado como *additional data*
    para autenticar e impedir troca de parâmetros.
  - Gravação atômica: arquivo temporário cifrado no mesmo diretório + `fsync` + `rename`;
    backup do arquivo anterior como `vault.bak` (também cifrado).
  - Bloqueio: DEK e dados decifrados são zerados (`sodium.memzero` nos buffers; estado JS
    descartado). Sessões abertas não são afetadas (já autenticadas).
  - Senha errada e arquivo adulterado produzem o mesmo erro genérico de desbloqueio; versão
    desconhecida e arquivo corrompido (magic/JSON inválido) dão erro próprio sem sobrescrever.
- **Rationale**: algoritmos exigidos pelo Princípio II, sem criptografia própria; envelope é o
  padrão de mercado para troca de senha barata.
- **Alternatives considered**: `node:crypto` com scrypt + AES-256-GCM (válido, mas nonce de
  96 bits aleatório tem margem menor e scrypt é mais fraco que Argon2id contra GPU);
  `safeStorage`/DPAPI (protege por usuário do Windows, não por senha mestra — contraria FR-002).

## R10. Ponte entre processos (IPC)

- **Decision**: preload expõe um único objeto `window.shellvault` com funções nomeadas
  (nunca `ipcRenderer` cru). Requisições via `ipcRenderer.invoke` com canal fixo por função;
  eventos de streaming (saída do terminal, progresso) via `ipcRenderer.on` com canais
  fixos e `sessionId` no payload. Toda entrada é validada com `zod` 4 (MIT) no processo
  principal; respostas usam um tipo `Result<T, AppError>` com código de erro, nunca stack
  trace nem segredos. Senhas digitadas trafegam só da interface para o principal; o principal
  nunca devolve segredos à interface (exceto chave pública, que não é segredo).
- Segurança extra: CSP estrita (`default-src 'self'`), `will-navigate` e `setWindowOpenHandler`
  bloqueados, `webSecurity` padrão, verificação do `senderFrame` em cada handler.
- **Rationale**: Princípio II (ponte mínima e validada).
- **Alternatives considered**: tRPC sobre IPC (dependência extra sem necessidade).

## R11. Testes

- **Decision**:
  - Unitários: Vitest 5 (MIT) — cofre/cripto, modelo de hosts/grupos, busca, validação de
    regras, parsing de chaves, tokens de contraste.
  - Integração: Vitest contra OpenSSH real em contêiner Docker (imagem própria em
    `tests/fixtures/sshd/Dockerfile`, Alpine + `openssh-server` + `sftp-server`, usuários de
    teste com senha, chave e keyboard-interactive). Requer Docker Desktop em execução.
  - E2E: Playwright 1.64 (Apache-2.0) com `_electron.launch` — criar cofre, conectar, abas.
  - Verificação de SC-004: teste que varre todos os arquivos do diretório de dados após um
    fluxo completo buscando as senhas/chaves de teste em texto claro.
- **Rationale**: Princípio IV exige servidor SSH real para SSH/SFTP/forwarding.
- **Alternatives considered**: `ssh2.Server` em processo (não é um servidor real; útil só
  para simular casos raros, ex.: troca de host key, se necessário).

## R12. Armazenamento de arquivos e privacidade

- **Decision**: diretório de dados `app.getPath('userData')` (`%APPDATA%\ShellVault`):
  `vault.svlt` (cifrado), `vault.bak` (cifrado), `settings.json` (só preferências não
  sensíveis: tema, fonte, tamanho da fonte, tamanho/posição da janela). Nenhum log em arquivo
  (FR-030): `crashReporter` desligado, sem `electron-log`, `console` apenas em
  desenvolvimento. Cache HTTP/GPU do Chromium não contém segredos; `session` sem
  persistência de storage web.
- **Rationale**: FR-002 lista o que precisa ser cifrado; preferências visuais precisam estar
  disponíveis antes do desbloqueio (tela de bloqueio já com tema certo).
- **Alternatives considered**: tudo dentro do cofre (tela de desbloqueio sem tema/fonte do
  usuário).

## R13. Internacionalização

- **Decision**: textos da interface em um único módulo `src/renderer/i18n/pt-BR.ts`
  (objeto tipado). Sem biblioteca de i18n.
- **Rationale**: FR-029 exige só pt-BR; YAGNI. Centralizar facilita adicionar idiomas depois.
