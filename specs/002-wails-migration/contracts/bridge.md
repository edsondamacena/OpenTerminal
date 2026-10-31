# Contrato: ponte interface ↔ Go (Wails bindings + eventos)

**Feature**: `002-wails-migration` | Substitui `specs/001-mvp-ssh-client/contracts/ipc-bridge.md`

O contrato funcional é o da 001: mesmas operações, os mesmos `ErrorCode` e as mesmas regras de
segredo. Esta versão define como ele é exposto no Wails e lista o que é **novo** ou **alterado**.
Operações sem marcação mantêm assinatura e semântica da 001.

## Regras gerais

- Cada grupo de operações é uma struct Go ligada ao Wails (`Bind`): `VaultService`,
  `HostService` (hosts e grupos), `CredentialService`, `SessionService`, `SftpService`,
  `ForwardService`, `ProfileService` **novo**, `AppService`. Nenhuma outra struct é ligada.
  Métodos exportados que não fazem parte deste contrato MUST NOT existir nessas structs (teste
  de reflexão compara a lista de métodos com este documento).
- A interface recebe sempre `Result<T>` =
  `{ ok: true, value: T } | { ok: false, error: { code: ErrorCode, message: string, op: string, detail: string } }`.
  Em Go, cada método devolve `(T, error)` (o gerador de bindings do Wails v2 não tipa
  genéricos) e todo erro passa por `apperr.Sanitize`: vira `*apperr.Error`, cujo texto é o JSON
  `{code, message, op, detail}`. `message` é fixa e curta em pt-BR; `op` identifica
  `Serviço.Método`; `detail` preserva o encadeamento técnico depois de remover senhas,
  passphrases, tokens e chaves privadas. Pânicos recuperados usam `INTERNAL` e colocam somente
  a mensagem sanitizada do pânico em `detail`, nunca a pilha. O adaptador
  `frontend/src/bridge` converte a promessa rejeitada em `{ ok: false, error }`; nenhuma
  exceção chega aos componentes.
- Toda entrada é validada no Go (tipos + regras do data-model); falha → `INVALID_INPUT`.
- Métodos 🔒 exigem cofre desbloqueado (`VAULT_LOCKED`).
- Segredos só trafegam interface → Go. Nenhum método devolve senha, passphrase ou chave privada.
- Caminhos locais só são escolhidos por diálogo nativo aberto pelo Go
  (`runtime.OpenFileDialog`/`OpenMultipleFilesDialog`/`OpenDirectoryDialog`); a interface
  nunca envia caminho local, exceto na importação de esquema de cores, que também usa diálogo.
- Eventos Go → interface: `runtime.EventsEmit(nome, payload)` com os nomes fixos abaixo; a
  interface registra **um** ouvinte por nome na inicialização (R10).

## ErrorCode

Os da 001 mais **novos**: `NO_COMMON_ALGORITHM` (FR-122), `AGENT_UNAVAILABLE` (FR-124),
`SCHEME_INVALID` (importação de esquema), `ALREADY_EXISTS` (nome já usado),
`IN_USE` (excluir esquema/perfil em uso),
`VAULT_MIGRATION_FAILED` (FR-102/103; o arquivo original permanece intacto).

## VaultService

Como `vault.*` da 001, com:

| Método | Mudança |
|---|---|
| `GetStatus()` | **alterado**: `state` ganha `"migratable"` — existe `vault.svlt` da 001 e ainda não há `shellvault.db` |
| `Unlock(password)` | **alterado**: no estado `migratable`, desbloqueia e migra (R5); falha da migração → `VAULT_MIGRATION_FAILED` e estado volta a `migratable` |
| `GetSettings()` / `UpdateSettings(patch)` 🔒 | **alterado**: `VaultSettings` inclui `connectTimeoutSeconds` (5–120), `keepaliveSeconds`, `legacyDefault` e `term` (`xterm-256color` \| `xterm` \| `vt100` \| `linux`) |

Evento `vault:state`: `VaultStatus` a cada mudança de estado (equivale ao `onStateChanged` da 001).

## HostService 🔒

Como `hosts.*`/`groups.*` da 001. **Alterado**: `HostInput`/`HostView` ganham
`authMethod: "agent"`, `legacyAlgorithms: boolean` e `profileId: string | null`.

## CredentialService 🔒

Igual à 001 (`List`, `CreatePassword`, `ImportKey`, `PickKeyFile`, `GenerateEd25519`,
`CopyPublicKey`, `Rename`, `Remove`).

## SessionService

| Método | Entrada | Saída | Notas |
|---|---|---|---|
| `OpenSsh(hostId)` 🔒 | — | `{ sessionId }` | **alterado**: sem tamanho; a sessão espera o primeiro `Resize` válido para pedir o PTY remoto (FR-113) |
| `OpenLocal(shell)` | `"powershell" \| "cmd"` | `{ sessionId }` | **alterado**: idem |
| `Write(sessionId, data)` | string | `void` | |
| `Resize(sessionId, cols, rows)` | ints 1–1000 | `void` | ignorado se igual ao último enviado |
| `Reconnect(sessionId)` 🔒 | | `void` | |
| `Close(sessionId)` | | `void` | retorna após cancelar o contexto e encerrar as goroutines da sessão; emite `term:closed` |
| `AnswerHostKey` / `AnswerPrompt` / `ActiveSummary` | | | iguais à 001 |
| `SendKeys(sessionId, combo)` **novo** | `"ctrl+w" \| "ctrl+t" \| "ctrl+tab" \| "ctrl+k"` | `void` | envia ao servidor combinações reservadas pelo app (Assumptions) |

Eventos:

| Evento | Payload | Regra |
|---|---|---|
| `term:data` | `{ sessionId, data }` | lotes de até 8 ms ou 32 KiB; texto UTF-8 completo (R10) |
| `term:state` | `{ sessionId, state, reason?, latencyMs }` | `latencyMs` é o RTT inteiro do último keepalive, ou `null` antes da primeira medição |
| `term:hostkey` | como `onHostKey` da 001 | |
| `term:prompt` | como `onPrompt` da 001 | |
| `term:closed` | `{ sessionId }` | último evento da sessão; nada é emitido depois |

## SftpService

Igual a `sftp.*` da 001. Eventos `sftp:transfer` e `sftp:conflict`.

## ForwardService

Igual a `forwards.*` da 001 (`ListActive` e `Deactivate` sem 🔒). Evento `forwards:status`.

## ProfileService **novo**

| Método | Entrada | Saída | Notas |
|---|---|---|---|
| `GetGlobal()` | — | `TerminalProfile` | sem 🔒 (terminais locais funcionam com cofre bloqueado) |
| `UpdateGlobal(patch)` | `TerminalProfile` parcial | `TerminalProfile` | emite `profiles:changed` |
| `GetForHost(hostId)` 🔒 | | `TerminalProfile \| null` | |
| `SetHostOverride(hostId, profile \| null)` 🔒 | perfil completo ou null para remover | `TerminalProfile \| null` | |
| `Effective(sessionId)` | | `TerminalProfile` | global ⊕ override do host da sessão |
| `ListSchemes()` | — | `ColorScheme[]` | |
| `SaveScheme(scheme)` | `ColorScheme` sem `origin: builtin` | `ColorScheme` | |
| `DuplicateScheme(id, name)` | | `ColorScheme` | |
| `RemoveScheme(id)` | | `void` | `IN_USE` se referenciado |
| `ImportScheme()` | — (diálogo nativo `.json`/`.itermcolors`) | `{ imported: ColorScheme[], completed: string[] }` \| null | `completed` lista cores ausentes preenchidas; `SCHEME_INVALID` com motivo |
| `ExportScheme(id)` | — (diálogo de salvar) | `void` | formato do data-model |
| `ListFonts()` | — | `{ family, source: "system" \| "bundled", ligatures: boolean }[]` | só monoespaçadas |

Evento `profiles:changed`: `{ scope: "global" | "host", hostId? }`.

## AppService

Como `app.*` da 001, com:

| Método / evento | Mudança |
|---|---|
| `GetAppearance()` / `app:appearance` | igual |
| `GetPreferences()` / `UpdatePreferences(patch)` | **renomeado** de `getUiSettings`; inclui `glassEnabled`, `defaultShell`, `collapsedGroups` e `shortcuts: Record<string, string>`; no patch, `shortcuts` substitui o mapa inteiro (`""` desativa a ação) |
| `app:close-requested` / `ConfirmClose(bool)` | igual à 001 (FR-028) |
| `WindowMinimise()`, `WindowToggleMaximise()`, `WindowClose()` **novo** | botões da barra de título própria (FR-133); `WindowClose` passa pela confirmação de FR-028 |
| `OpenExternalUrl(url)` **novo** | abre no navegador padrão; só `http`/`https`; chamado apenas por Ctrl+clique (FR-145) |

## TestService (somente build com tag `e2e`)

Não existe no binário de release. `Maximise()`, `Restore()`, `SetSize(w, h)`, `Goroutines()`,
`SessionCount()`, `ServerSizeOf(sessionId)`. Usado pelos testes E2E (research R18).
