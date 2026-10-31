# Revisão de segurança da migração 002

Data: 2026-10-25

Escopo: as 15 linhas da tabela “Revisão de segurança (Princípio II)” de
`specs/002-wails-migration/plan.md`, mais a sanitização dos detalhes de erro que atravessam a
ponte. A revisão do backend foi concluída; três controles de interface ainda precisam ser
implementados no frontend e foram repassados ao responsável.

| Risco | Evidência no código | Teste ou verificação | Status |
|---|---|---|---|
| Criptografia divergir da 001 | `internal/vault/crypto.go:40` usa Argon2id e `crypto.go:64` usa XChaCha20-Poly1305. | `internal/vault/crypto_test.go:36` compara a KEK com o vetor libsodium da 001; `internal/vault/migrate001/migrate001_test.go:67` abre e compara o fixture real. | OK |
| Linhas cifradas trocadas | `internal/vault/rows.go:141` forma AD como `tabela:id`; `crypto.go:75` autentica o AD. | `internal/vault/store_test.go:188` troca blobs entre ids e exige falha; `crypto_test.go:70` adultera AD, nonce e ciphertext. | OK |
| Metadados em claro | `internal/vault/schema.go:27-58` deixa em claro somente ids, relações, ordem e datas; os dados sensíveis ficam em `sealed`. | `internal/vault/store_test.go:251` varre todos os arquivos SQLite/WAL e procura rótulo, endereço, usuário, credencial, senha de host e senha mestra. | OK |
| Páginas apagadas e WAL | `internal/vault/store.go:114` ativa `secure_delete`; `store.go:332-338` zera a chave e executa checkpoint `TRUNCATE` ao bloquear. | `internal/vault/store_test.go:243` verifica o pragma; `store_test.go:76` verifica descarte e zeragem da DEK; `store_test.go:251` varre banco e WAL. | OK |
| Migração inconsistente | `internal/vault/migrate001/migrate001.go:170-193` grava transacionalmente, relê e compara, desfaz em falha e só então renomeia o original; `migrate001.go:196` faz a verificação. | `internal/vault/migrate001/migrate001_test.go:67`, `:169` e `:179` cobrem sucesso, senha errada e falha após importação, mantendo o original intacto e permitindo nova tentativa. | OK — a implementação usa transação + verificação + rollback seguro em vez do arquivo `.tmp` descrito no plano. |
| Algoritmos fracos | `internal/sshx/algorithms.go:12` parte dos defaults modernos e só acrescenta SHA-1/CBC por host. | `internal/sshx/sshx_test.go:453` recusa servidor legado por padrão; `internal/sshx/algorithms_test.go:33` confirma modernos antes dos legados. | OK. Frontend: `HostForm.tsx` expõe a opção e `HostList.tsx:330` mostra o selo permanente (conferido pelo Claude; o achado inicial veio de um checkout defasado). |
| Agente SSH malicioso ou ausente | `internal/sshx/agent_windows.go:13-29` limita produção a `\\.\pipe\openssh-ssh-agent`; não há agent forwarding no `ssh.ClientConfig`. | `internal/sshx/agent_windows_test.go:21` autentica pelo pipe; `:55` mapeia ausência para `AGENT_UNAVAILABLE`; `:67` prova que `SSH_AUTH_SOCK` é ignorado. | Corrigido nesta revisão: removido o aceite de `SSH_AUTH_SOCK` em produção. |
| Ponte expondo métodos demais | `internal/bridge/services.go` liga os serviços; o contrato fechado está em `internal/bridge/contract_test.go`. | `internal/bridge/contract_test.go:30` compara por reflexão os métodos dos oito serviços com a lista exata. | OK |
| Porta de depuração em produção | `main_e2e.go:19` acrescenta a porta somente sob a tag `e2e`; `scripts/check-release.mjs:10-20` procura os marcadores proibidos no executável de release. | `node scripts/check-release.mjs` é a verificação de release definida em T018; a suíte Go compila o caminho sem tag. | OK; a varredura binária deve continuar obrigatória no empacotamento. |
| URLs maliciosas no terminal | `internal/bridge/app.go:227` aceita somente `http`/`https` com host e delega ao navegador; `frontend/index.html:6` mantém CSP restritiva. | `internal/bridge/core_services_test.go:28` aceita HTTPS e recusa `file:`. | OK. Frontend: `TerminalView.tsx:86` carrega o `WebLinksAddon` e só chama `openExternalUrl` com `ctrlKey` (conferido pelo Claude). |
| Colagem de comandos perigosos | O backend limita cada escrita a 1 MiB, e `frontend/src/features/terminal/TerminalView.tsx:34` passa a colagem por `needsConfirm` antes do xterm. | `frontend/src/features/terminal/paste.test.ts` cobre uma linha com quebra final e várias linhas com controle escapado. | OK. Frontend: `paste.ts` (`needsConfirm`) e `PasteConfirmDialog.tsx`, com `paste.test.ts` (T055/T063, conferido pelo Claude). |
| Segredos residuais em memória | `internal/vault/store.go:194-230` zera KEKs com `defer`; `store.go:332-338` zera a DEK no bloqueio; `internal/vault/crypto.go:91` usa `clear`. Strings permanecem como risco aceito no plano. | `internal/vault/crypto_test.go:102` cobre `Zero`; `internal/vault/store_test.go:76` agora conserva um alias e confirma que os bytes da DEK foram zerados. | OK, com o risco aceito de strings não zeráveis. |
| Crash dumps do WebView2 | `internal/appwin/datadir.go:49` fixa `--disable-breakpad` e `--disable-crash-reporter`; `webview_windows.go:8-9` os entrega ao loader antes de iniciar. | `internal/appwin/datadir_test.go:29` verifica os dois argumentos. | OK |
| Autofill do WebView2 persistir formulários e chaves em claro | `internal/appwin/webview_windows.go:9` ativa `edge.DisableAutofill`; `third_party/go-webview2/pkg/edge/chromium.go:368-374` aplica os dois setters de `ICoreWebView2Settings4` antes da navegação. `internal/appwin/datadir.go:38` remove `Web Data` e sidecars legados antes de criar o WebView. | `internal/appwin/webview_windows_test.go:11` exige a opção ativa; `internal/appwin/datadir_test.go:39` confirma que os quatro arquivos são removidos sem afetar outro arquivo. O E2E `secrets-scan.spec.ts` inspeciona o SQLite real. | Corrigido nesta revisão; E2E real encaminhado ao Claude. |
| Shell falso no PATH | `internal/localpty/shells.go` resolve Windows PowerShell e cmd diretamente em `%SystemRoot%\\System32`; WSL também prefere `System32`. | `internal/localpty/pty_integration_test.go:75` exige caminho absoluto dentro de `%SystemRoot%`; `shells_test.go` confirma a preferência do WSL e que resolver cmd não consulta PATH/WSL. | OK |

## Achado adicional: detalhes de erro

`apperr.Sanitize` preserva contexto técnico em `detail`, portanto a remoção de segredos precisa
cobrir mais do que atribuições `chave=valor`. Nesta revisão, `internal/apperr/apperr.go` passou a
remover também credenciais em URLs, cabeçalhos Basic/Bearer, flags de linha de comando, chaves de
API/acesso e prefixos conhecidos de tokens, além de PEMs e atribuições já cobertos.
`internal/apperr/apperr_test.go:58` contém a regressão com todas essas formas. O limite de 2.048
caracteres continua aplicado.

## Pendências encaminhadas ao frontend

Nenhuma: os três pontos levantados já estavam implementados no `002-wails-migration`; a revisão
inicial leu um frontend defasado no worktree do backend.
