# Data Model: MVP do cliente SSH ShellVault

**Feature**: `001-mvp-ssh-client` | **Date**: 2026-10-23

Identificadores são UUID v4 (`crypto.randomUUID()`). Datas em ISO 8601 UTC. Entidades
persistidas ficam dentro do conteúdo cifrado do cofre, exceto `Settings` e o cabeçalho do
`VaultFile`.

## Arquivo do cofre (`vault.svlt`)

Formato binário com cabeçalho em claro autenticado:

| Campo | Tipo | Regra |
|---|---|---|
| `magic` | 4 bytes | `SVLT` |
| `formatVersion` | uint16 | `1` no MVP; versão maior que a suportada → erro "versão desconhecida", arquivo intocado |
| `headerLength` | uint32 | tamanho do cabeçalho JSON |
| `header` | JSON UTF-8 | ver abaixo; é *additional data* das duas cifragens |
| `payload` | bytes | `nonce(24) ‖ XChaCha20-Poly1305(DEK, VaultContent JSON)` |

`header`:

```text
kdf:        { alg: "argon2id13", opsLimit: int, memLimit: int, salt: base64(16) }
wrappedDek: base64( nonce(24) ‖ XChaCha20-Poly1305(KEK, DEK(32)) )
createdAt:  ISO date
```

**Migração** (Princípio III): duas versões independentes.

- `formatVersion` (envelope binário/cabeçalho): cada versão nova MUST ter
  `migrateFormat_vN_to_vN+1` com teste.
- `schemaVersion` (conteúdo decifrado): cada incremento MUST ter
  `migrateContent_vN_to_vN+1` com teste, executada após decifrar e antes da validação zod.

Em ambos os casos o arquivo original é preservado como `vault.vN.bak` (`N` = versão
anterior) antes de gravar a versão migrada.

## VaultContent (conteúdo decifrado)

```text
schemaVersion:  int (1)
settings:       VaultSettings
groups:         Group[]
hosts:          Host[]
credentials:    Credential[]
knownHostKeys:  KnownHostKey[]
forwardRules:   ForwardRule[]
```

### VaultSettings

| Campo | Tipo | Regra |
|---|---|---|
| `autoLockMinutes` | int \| null | 1–120; `null` = desativado; padrão 15 (FR-003) |
| `connectTimeoutSeconds` | int | 5–120; padrão 15 |

### Group

| Campo | Tipo | Regra |
|---|---|---|
| `id` | uuid | |
| `name` | string | 1–100 caracteres, aparado; nomes iguais permitidos em pais diferentes |
| `parentId` | uuid \| null | MUST existir; MUST NOT criar ciclo (validado ao mover) |
| `sortOrder` | int | ordem entre irmãos |

Excluir grupo (FR-010, US3-4): seus hosts e subgrupos sobem para a raiz (`groupId = null`,
`parentId = null`). Não há exclusão em cascata.

### Host

| Campo | Tipo | Regra |
|---|---|---|
| `id` | uuid | |
| `label` | string | 1–100 caracteres; duplicados permitidos |
| `address` | string | hostname (RFC 1123), IPv4 ou IPv6 (com ou sem colchetes; normalizado sem) |
| `port` | int | 1–65535; padrão 22 |
| `username` | string | 1–255 caracteres, sem espaços nem controle |
| `authMethod` | `"password"` \| `"key"` \| `"ask"` | FR-007 |
| `credentialId` | uuid \| null | obrigatório se `authMethod` ≠ `"ask"`; tipo da credencial MUST combinar (`password`→`PasswordCredential`, `key`→`KeyCredential`) |
| `groupId` | uuid \| null | |
| `createdAt`, `updatedAt` | ISO date | |

Duplicar (US3-5): cópia com novo `id`, rótulo `"<rótulo> (cópia)"`, mesma credencial
referenciada, regras de encaminhamento copiadas com novos ids. Excluir host remove suas
`ForwardRule`; credenciais não são removidas (podem ser compartilhadas).

Busca (FR-011): `label`, `address`, `username` normalizados com `toLocaleLowerCase('pt-BR')`
e remoção de acentos (NFD); casa por substring. Índice em memória recalculado a cada mudança.

### Credential (união discriminada por `kind`)

Comuns: `id`, `name` (1–100), `createdAt`.

**PasswordCredential** — `kind: "password"`, `password: string` (1–1024).

**KeyCredential** — `kind: "key"`:

| Campo | Tipo | Regra |
|---|---|---|
| `privateKey` | string | conteúdo OpenSSH ou PEM validado por `ssh2.utils.parseKey` na importação |
| `passphrase` | string \| null | MUST abrir a chave na importação; senão nada é salvo |
| `keyType` | `"ssh-ed25519"` \| `"ecdsa-sha2-nistp256/384/521"` \| `"ssh-rsa"` | derivado |
| `publicKey` | string | formato `authorized_keys`, derivado |
| `fingerprint` | string | `SHA256:…` da chave pública |
| `origin` | `"imported"` \| `"generated"` | |

Credenciais nunca saem do processo principal; a interface recebe `CredentialSummary`
(`id`, `name`, `kind`, `keyType?`, `publicKey?`, `fingerprint?`, `usedByHostCount`).

### KnownHostKey

| Campo | Tipo | Regra |
|---|---|---|
| `endpoint` | string | chave única `"<address normalizado>:<port>"` |
| `keyType` | string | ex.: `ssh-ed25519` |
| `fingerprint` | string | `SHA256:<base64 sem padding>` |
| `acceptedAt` | ISO date | |

Regras (FR-013): sem registro → pedir confirmação, gravar ao aceitar. Registro com mesmo
`keyType` e `fingerprint` diferente → bloquear com alerta de divergência; aceitar substitui.
Servidor apresentando tipo diferente do registrado também é tratado como divergência.

### ForwardRule

| Campo | Tipo | Regra |
|---|---|---|
| `id` | uuid | |
| `hostId` | uuid | MUST existir |
| `label` | string \| null | |
| `bindAddress` | string | padrão `127.0.0.1`; outro valor exige `allowNonLoopback: true` (FR-025) |
| `allowNonLoopback` | boolean | confirmado na interface com aviso |
| `bindPort` | int | 1–65535; único entre regras com mesmo `bindAddress` |
| `destHost` | string | hostname/IP resolvido pelo servidor |
| `destPort` | int | 1–65535 |

O estado de ativação não é persistido (todas começam inativas a cada execução).

## Settings (`settings.json`, em claro, sem segredos)

| Campo | Tipo | Padrão |
|---|---|---|
| `theme` | `"system"` \| `"light"` \| `"dark"` | `"system"` |
| `terminalFontFamily` | string | `"Cascadia Mono, Consolas, monospace"` |
| `terminalFontSize` | int 8–32 | 14 |
| `window` | `{ width, height, x?, y?, maximized }` | 1200×780 |

Validado com zod ao carregar; arquivo inválido → padrões (não é dado sensível).

## Entidades em memória (não persistidas)

### VaultState

```text
nonexistent ──create──▶ unlocked
locked ──unlock(senha ok)──▶ unlocked
unlocked ──lock (manual | inatividade)──▶ locked
locked|unlocked ──erase (confirmação dupla)──▶ nonexistent
(arquivo corrompido/versão desconhecida) ──▶ unreadable  (só permite erase)
```

Inatividade: qualquer evento de teclado/mouse na janela (inclusive no terminal) reinicia o
timer no processo principal (throttle de 30 s).

### Session (aba)

| Campo | Tipo |
|---|---|
| `id` | uuid |
| `kind` | `"ssh"` \| `"local"` \| `"sftp"` |
| `title` | string (renomeável) |
| `hostId` | uuid \| null |
| `shell` | `"powershell"` \| `"cmd"` \| null |
| `state` | ver abaixo |
| `order` | int |

```text
ssh/sftp: connecting ─▶ awaitingHostKey ─▶ authenticating ─▶ (awaitingPrompt ⇄ authenticating) ─▶ connected
          qualquer ─erro/queda─▶ disconnected(reason) ─reconnect─▶ connecting   (reconnect exige cofre desbloqueado)
local:    running ─exit(code)─▶ exited(code)
```

### Transfer

| Campo | Tipo |
|---|---|
| `id` | uuid |
| `sessionId` | uuid (aba SFTP) |
| `direction` | `"upload"` \| `"download"` |
| `localPath`, `remotePath` | string |
| `bytesTotal`, `bytesDone` | number |
| `state` | `queued` → `running` → `completed` \| `cancelled` \| `failed(reason)` \| `skipped` |
| `conflict` | `null` \| `pending` → `replace` \| `skip` \| `cancelAll` |

Arquivo parcial (FR-021): grava em `<nome>.svpart-<id8>` no destino e renomeia ao concluir;
falha/cancelamento apaga o parcial.

### ForwardRuleStatus

`inactive` → `starting` → `active` | `error(reason: "portInUse" | "connectionFailed" | …)`;
`active` → `inactive` ao desativar ou ao cair a conexão (porta liberada, FR-026).
