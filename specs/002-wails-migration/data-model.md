# Data Model: Migração do ShellVault para Go + Wails

**Feature**: `002-wails-migration` | **Date**: 2026-10-23

As entidades de negócio são as mesmas da 001 ([data-model 001](../001-mvp-ssh-client/data-model.md))
com os acréscimos marcados como **novo**. Muda o contêiner: de um arquivo `.svlt` único para um
banco SQLite com conteúdo cifrado por linha (research R3, R4).

## Arquivo `shellvault.db`

`PRAGMA user_version` = versão do esquema (1 nesta feature). Toda mudança de esquema MUST ter
migração numerada com teste (Princípio III).

### `vault_meta` (uma linha)

| Coluna | Tipo | Regra |
|---|---|---|
| `format_version` | int | 1 |
| `kdf_alg` | text | `argon2id` |
| `kdf_time` | int | passes (padrão 3) |
| `kdf_memory_kib` | int | padrão 262144 (256 MiB) |
| `kdf_salt` | blob(16) | aleatório |
| `wrapped_dek` | blob | `nonce(24) ‖ XChaCha20-Poly1305(KEK, DEK)`, AD = `"shellvault:dek:v1" ‖ kdf_*` serializados |
| `created_at` | text | ISO 8601 UTC |
| `migrated_from` | text \| null | `"vault.svlt v1"` quando veio da 001 |

Trocar a senha mestra (FR-004) re-embrulha a DEK com KEK nova (sal novo); as linhas não são
recifradas.

### Tabelas cifradas por linha

Formato comum: colunas estruturais em claro + `sealed blob` =
`nonce(24) ‖ XChaCha20-Poly1305(DEK, JSON da entidade)`, AD = `"<tabela>:<id>"`.

| Tabela | Colunas em claro | Conteúdo de `sealed` |
|---|---|---|
| `groups` | `id`, `parent_id` (FK, null), `sort_order` | `{name}` |
| `hosts` | `id`, `group_id` (FK, null), `sort_order`, `created_at`, `updated_at` | `{label, address, port, username, authMethod, credentialId, legacyAlgorithms, profileId}` |
| `credentials` | `id`, `kind`, `created_at` | password: `{name, password}`; key: `{name, privateKey, passphrase, keyType, publicKey, fingerprint, origin}` |
| `known_hosts` | `id` | `{endpoint, keyType, fingerprint, acceptedAt}` |
| `forward_rules` | `id`, `host_id` (FK, cascade) | `{label, bindAddress, allowNonLoopback, bindPort, destHost, destPort}` |
| `vault_settings` | `id` (=1) | `{autoLockMinutes, connectTimeoutSeconds, keepaliveSeconds}` |

`known_hosts.endpoint` fica cifrado; a busca por endpoint é feita no índice em memória
montado após o desbloqueio (o conteúdo inteiro é decifrado no desbloqueio, como na 001; o
volume esperado — ~1.000 hosts — cabe em memória).

### Tabelas em claro (sem segredos)

| Tabela | Colunas | Uso |
|---|---|---|
| `preferences` | `key` text PK, `value` json | tema (`system`/`light`/`dark`), vidro ligado, shell padrão, estado da janela, grupos recolhidos (por id), última versão vista |
| `terminal_profiles` | `id`, `name`, `is_global` bool, `data` json | ver TerminalProfile |
| `color_schemes` | `id`, `name`, `origin`, `data` json | ver ColorScheme |

Grupos recolhidos guardam só ids (não revelam nomes).

## Entidades (mudanças em relação à 001)

### Host (alterado)

| Campo | Tipo | Regra |
|---|---|---|
| `authMethod` | `"password"` \| `"key"` \| `"ask"` \| `"agent"` **novo** | `agent` não usa `credentialId` (MUST ser null) |
| `legacyAlgorithms` **novo** | boolean | padrão `false` (FR-121) |
| `profileId` **novo** | uuid \| null | MUST referenciar `terminal_profiles` com `is_global = false`; null = usa o global |
| `detectedOs` **novo** | id de sistema \| null | gravado pela detecção (FR-178); null = desconhecido; não editável pela ponte de hosts |
| `osIcon` **novo** | id de sistema \| `"auto"` | escolha do usuário; padrão `"auto"` = usa `detectedOs` |

Ids de sistema (lista fechada, mesma no Go e no frontend): `ubuntu`, `debian`, `centos`, `rhel`,
`fedora`, `rocky`, `almalinux`, `alpine`, `arch`, `opensuse`, `oracle`, `amazon`, `linux`
(Linux sem distribuição reconhecida), `freebsd`, `openbsd`, `windows`, `macos`, `mikrotik`,
`cisco`, `huawei`, `juniper`, `hpe` (Comware/Aruba), `fortinet`, `openwrt`.

Demais campos e regras como na 001 (label 1–100, endereço RFC 1123/IPv4/IPv6, porta 1–65535,
usuário 1–255 etc.). Duplicar copia `legacyAlgorithms`, `profileId`, `detectedOs` e `osIcon`.

### VaultSettings (alterado)

`keepaliveSeconds` **novo**: int, 10–300, padrão 30 (FR-120). Falhas para desconectar: 3 (fixo).

### TerminalProfile **novo** (FR-152, FR-156)

| Campo | Tipo | Regra / padrão |
|---|---|---|
| `fontFamily` | string | família instalada ou embutida; padrão `JetBrains Mono` |
| `fontSize` | number | 8–32, padrão 14 |
| `fontWeight` / `fontWeightBold` | int | 100–900; 400 / 700 |
| `lineHeight` | number | 1.0–2.0, padrão 1.2 |
| `letterSpacing` | number | −2 a 10 px, padrão 0 |
| `ligatures` | boolean | padrão false |
| `cursorStyle` | `block` \| `bar` \| `underline` | `block` |
| `cursorBlink` | boolean | true |
| `cursorColor` | cor \| null | null = do esquema |
| `scrollback` | int | 100–100.000, padrão 10.000 |
| `backgroundOpacity` | number | 0.5–1.0, padrão 1.0 (FR-150) |
| `blur` | int | 0–40 px, padrão 0 (só tem efeito com opacidade < 1) |
| `padding` | int | 0–32 px, padrão 12 |
| `colorSchemeId` | uuid | MUST existir |

Perfil global: exatamente um com `is_global = true`, criado na primeira execução. Perfil de
host: cópia completa (não diferencial) criada ao ativar o override; desativar o override
remove o perfil e zera `host.profileId`. Zoom (Ctrl+=/−/0) muda só o `fontSize` efetivo da
aba, sem persistir.

### ColorScheme **novo** (FR-153–FR-155)

| Campo | Tipo | Regra |
|---|---|---|
| `name` | string | 1–100; único |
| `origin` | `builtin` \| `imported` \| `custom` | `builtin` não é editável nem excluível (duplicar para editar) |
| `foreground`, `background`, `cursor`, `selection` | cor `#rrggbb` (`selection` aceita `#rrggbbaa`) | obrigatórias |
| `ansi` | 16 cores `#rrggbb` | ordem black, red, green, yellow, blue, magenta, cyan, white, brightBlack…brightWhite |

Excluir um esquema em uso por um perfil MUST ser recusado com mensagem listando os perfis.

### Formato de exportação de esquema (FR-155)

JSON do próprio ShellVault, compatível com a forma de esquema do Windows Terminal (assim
o arquivo exportado pode ser importado no Windows Terminal também):

```text
{ "name", "foreground", "background", "cursorColor", "selectionBackground",
  "black", "red", "green", "yellow", "blue", "purple", "cyan", "white",
  "brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue",
  "brightPurple", "brightCyan", "brightWhite" }
```

## Entidades em memória

### Session (alterado)

| Campo | Tipo | Regra |
|---|---|---|
| `id` | uuid | |
| `kind` | `ssh` \| `local` \| `sftp` | |
| `state` | `connecting` \| `authenticating` \| `awaitingHostKey` \| `awaitingPrompt` \| `connected` \| `disconnected` \| `error` \| `closed` | `running`/`exited` da 001 viram `connected`/`closed` para sessões locais |
| `cols`, `rows` | int | > 0; só enviados ao servidor após o primeiro fit válido |
| `hostId` | uuid \| null | |
| `reason` | código de erro \| null | em `disconnected`/`error` |

Transições: `connecting → authenticating → (awaitingHostKey | awaitingPrompt)* → connected →
disconnected | closed`; `error` a partir de qualquer estado anterior a `connected`.
`closed` é terminal: nenhum evento é emitido depois de `term:closed`.

### Transfer, ForwardRuleStatus, VaultState

Iguais à 001.

## Migração da 001 → 002 (research R5)

| 001 (`vault.svlt` + `settings.json`) | 002 (`shellvault.db`) |
|---|---|
| `VaultContent.groups[]` | `groups` (nome → `sealed`) |
| `hosts[]` | `hosts` (+ `legacyAlgorithms: false`, `profileId: null`) |
| `credentials[]` | `credentials` |
| `knownHostKeys[]` | `known_hosts` (id novo) |
| `forwardRules[]` | `forward_rules` |
| `settings` (autoLock, connectTimeout) | `vault_settings` (+ `keepaliveSeconds: 30`) |
| `settings.json` (tema, fonte, tamanho, janela) | `preferences` + perfil global (fonte/tamanho) |
| cabeçalho KDF, DEK | descartados; KEK/DEK novas |

Ids da 001 são preservados (mesmos UUIDs), exceto `known_hosts`, que não tinha id.
