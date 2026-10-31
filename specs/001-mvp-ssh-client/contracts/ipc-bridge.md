# Contrato: ponte IPC (`window.shellvault`)

**Feature**: `001-mvp-ssh-client` | **Date**: 2026-10-23

Única superfície entre a interface (sem privilégios) e o processo principal (Princípio II,
research R10). Os tipos TypeScript desta API vivem em `src/shared/ipc/` e são a fonte da
verdade; este documento descreve o contrato e as regras.

## Regras gerais

- Toda função retorna `Promise<Result<T>>`:
  `{ ok: true, value: T } | { ok: false, error: { code: ErrorCode, message: string } }`.
  `message` é texto pt-BR pronto para a interface; MUST NOT conter segredos, caminhos de
  chave privada com conteúdo, stack trace ou resposta crua do servidor além do texto de
  erro de autenticação que o servidor enviou.
- Toda entrada é validada com zod no principal; falha de validação → `INVALID_INPUT`.
- Handlers verificam que o `senderFrame` é a janela principal do app.
- Funções marcadas 🔒 exigem cofre desbloqueado; caso contrário → `VAULT_LOCKED`.
- Segredos só trafegam interface → principal (senha digitada, passphrase, conteúdo de chave
  importada). O principal nunca devolve segredos.
- Eventos: `on<Evento>(handler) => unsubscribe`. Canais fixos; payload sempre com ids.

## ErrorCode

`INVALID_INPUT`, `VAULT_LOCKED`, `VAULT_NOT_FOUND`, `VAULT_EXISTS`, `VAULT_BAD_PASSWORD`
(também usado para adulteração), `VAULT_UNREADABLE` (corrompido), `VAULT_UNSUPPORTED_VERSION`,
`WEAK_PASSWORD`, `NOT_FOUND`, `GROUP_CYCLE`, `KEY_UNSUPPORTED`, `KEY_BAD_PASSPHRASE`,
`CONNECT_TIMEOUT`, `CONNECT_REFUSED`, `HOST_UNREACHABLE`, `AUTH_FAILED`,
`HOST_KEY_REJECTED`, `SESSION_CLOSED`, `SFTP_ERROR`, `PERMISSION_DENIED`, `PORT_IN_USE`,
`CANCELLED`, `INTERNAL`.

## vault

| Função | Entrada | Saída | Notas |
|---|---|---|---|
| `getStatus()` | — | `{ state: "nonexistent" \| "locked" \| "unlocked" \| "unreadable", reason? }` | |
| `create(password, confirmation)` | strings | `void` | ≥ 8 caracteres e iguais; senão `WEAK_PASSWORD`/`INVALID_INPUT` |
| `unlock(password)` | string | `void` | erro genérico `VAULT_BAD_PASSWORD` |
| `lock()` | — | `void` | |
| `changePassword(current, next, confirmation)` 🔒 | strings | `void` | |
| `erase(confirmPhrase)` | string = `"APAGAR"` | `void` | segunda confirmação; a primeira é diálogo na interface |
| `getSettings()` / `updateSettings(patch)` 🔒 | `VaultSettings` parcial | `VaultSettings` | |
| `reportActivity()` | — | `void` | throttled na interface (30 s) |
| evento `onStateChanged` | | `{ state }` | inclui bloqueio por inatividade |

## hosts / groups 🔒

| Função | Entrada | Saída |
|---|---|---|
| `hosts.list()` | — | `HostView[]` (Host + `credentialName?`) |
| `hosts.create(input)` | `HostInput` (Host sem id/datas) | `HostView` |
| `hosts.update(id, input)` | | `HostView` |
| `hosts.duplicate(id)` | | `HostView` |
| `hosts.remove(id)` | | `void` |
| `hosts.move(id, groupId \| null)` | | `void` |
| `groups.list()` | — | `Group[]` |
| `groups.create(name, parentId \| null)` | | `Group` |
| `groups.rename(id, name)` | | `Group` |
| `groups.move(id, parentId \| null)` | | `void` (`GROUP_CYCLE`) |
| `groups.remove(id)` | | `void` (filhos sobem para a raiz) |

A busca (FR-011) roda na interface sobre `HostView[]` (sem segredos), para atender SC-003
sem ida e volta por tecla.

## credentials 🔒

| Função | Entrada | Saída |
|---|---|---|
| `credentials.list()` | — | `CredentialSummary[]` |
| `credentials.createPassword(name, password)` | | `CredentialSummary` |
| `credentials.importKey(name, { filePath } \| { pem }, passphrase?)` | caminho escolhido em diálogo do principal, ou texto colado | `CredentialSummary` (`KEY_UNSUPPORTED`, `KEY_BAD_PASSPHRASE`) |
| `credentials.pickKeyFile()` | — | `{ filePath } \| null` (diálogo nativo no principal) |
| `credentials.generateEd25519(name, passphrase?)` | | `CredentialSummary` com `publicKey` |
| `credentials.copyPublicKey(id)` | | `void` (principal escreve na área de transferência; ação explícita) |
| `credentials.rename(id, name)` / `remove(id)` | | remove falha com `INVALID_INPUT` se em uso |

## sessions (terminal SSH e local)

| Função | Entrada | Saída | Notas |
|---|---|---|---|
| `sessions.openSsh(hostId, size)` 🔒 | `size = { cols, rows }` ints 1–1000 | `{ sessionId }` | estado inicial `connecting` |
| `sessions.openLocal(shell, size)` | `"powershell" \| "cmd"`, `size` | `{ sessionId }` | não exige cofre (FR-017) |
| `sessions.write(sessionId, data)` | string | `void` | entrada do teclado |
| `sessions.resize(sessionId, cols, rows)` | ints 1–1000 | `void` | |
| `sessions.reconnect(sessionId)` 🔒 | | `void` | vale para abas `ssh` e `sftp` |
| `sessions.close(sessionId)` | | `void` | |
| `sessions.answerHostKey(sessionId, accept)` | boolean | `void` | |
| `sessions.answerPrompt(sessionId, answers)` | `string[]` | `void` | keyboard-interactive / senha "ask" / passphrase |
| `sessions.activeSummary()` | — | `{ sessions, transfers, forwards }` contagens | para FR-028 |
| evento `onData` | | `{ sessionId, data }` | saída do terminal |
| evento `onState` | | `{ sessionId, state, reason? }` | ver data-model |
| evento `onHostKey` | | `{ sessionId, kind: "new" \| "changed", keyType, fingerprint, previousFingerprint? }` | |
| evento `onPrompt` | | `{ sessionId, name, instructions, prompts: { text, echo }[] }` | |

Saída do terminal é agrupada no principal (lotes a cada 8 ms ou 64 KiB) para não saturar o IPC.

## sftp

| Função | Entrada | Saída |
|---|---|---|
| `sftp.open(hostId)` 🔒 | | `{ sessionId, homePath }` (mesmo fluxo de host key/prompts de `sessions`) |
| `sftp.list(sessionId, path)` | | `RemoteEntry[]` `{ name, path, size, modifiedAt, type: "file" \| "dir" \| "link" \| "other" }` |
| `sftp.mkdir(sessionId, path)` / `rename(sessionId, from, to)` / `remove(sessionId, path, recursive)` | | `void` |
| `sftp.upload(sessionId, remoteDir)` | — (principal abre diálogo de seleção múltipla) | `{ transferIds }` |
| `sftp.download(sessionId, remotePaths)` | | `{ transferIds }` (principal abre diálogo de pasta destino) |
| `sftp.resolveConflict(transferId, choice)` | `"replace" \| "skip" \| "cancelAll"` | `void` |
| `sftp.cancel(transferId)` | | `void` |
| evento `onTransfer` | | `Transfer` (progresso throttled a 10/s) |
| evento `onConflict` | | `{ transferId, name, existingSize, existingModifiedAt }` |

Caminhos locais são escolhidos sempre por diálogo nativo no principal; a interface nunca
informa caminho local arbitrário (reduz superfície da ponte).

## forwards 🔒 (exceto desativar)

| Função | Entrada | Saída |
|---|---|---|
| `forwards.list(hostId)` | | `(ForwardRule & { status })[]` |
| `forwards.listActive()` (sem 🔒) | — | `{ ruleId, hostLabel, bindAddress, bindPort, destHost, destPort, status }[]` — só regras não inativas; nenhum dado é segredo (FR-003: encaminhamentos abertos continuam visíveis com cofre bloqueado) |
| `forwards.create(input)` / `update(id, input)` / `remove(id)` | `ForwardRuleInput` | `ForwardRule` |
| `forwards.activate(id)` | | `void` (`PORT_IN_USE`, erros de conexão) |
| `forwards.deactivate(id)` | | `void` (permitido com cofre bloqueado) |
| evento `onStatus` | | `{ ruleId, status, reason? }` |

## app

| Função | Saída |
|---|---|
| `app.getAppearance()` | `{ glass: boolean, highContrast: boolean, dark: boolean }` |
| evento `onAppearanceChanged` | mesmo objeto (mudança de transparência/alto contraste/tema do Windows) |
| `app.getUiSettings()` / `updateUiSettings(patch)` | `Settings` (`settings.json`, sem cofre) |
| evento `onCloseRequested` | principal pergunta à interface se há atividade; interface mostra confirmação e chama `app.confirmClose(true \| false)` |
