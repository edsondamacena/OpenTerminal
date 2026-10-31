# Implementation Plan: MVP do cliente SSH ShellVault

**Branch**: `001-mvp-ssh-client` | **Date**: 2026-10-23 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/001-mvp-ssh-client/spec.md`

## Summary

Cliente SSH desktop para Windows, local-first: cofre cifrado com senha mestra (Argon2id +
XChaCha20-Poly1305, envelope de chave), hosts com grupos aninhados e busca, terminal SSH e
local em abas, SFTP com transferências seguras e port forwarding local. Implementado em
Electron com três camadas: processo principal (cofre, SSH, PTY, arquivos), interface React
isolada e sem privilégios, e uma ponte IPC tipada e validada. A moldura usa o material
acrílico do Windows 11 com fallback sólido automático. Decisões detalhadas em
[research.md](research.md).

## Technical Context

**Language/Version**: TypeScript 6.0 (`strict`), executado no Node do Electron 45; Node 24 LTS
para ferramentas

**Primary Dependencies**: Electron 45, electron-vite 6, electron-builder 26, React 19,
Zustand 5, `@xterm/xterm` 6 (+ addons fit, webgl, unicode11), `ssh2` 1.17, `node-pty` 1.1,
`libsodium-wrappers-sumo` 0.8, `zod` 4 — todas MIT/ISC/Apache-2.0

**Storage**: arquivos locais em `%APPDATA%\ShellVault`: `vault.svlt` (cifrado, formato
versionado), `vault.bak` (cifrado), `settings.json` (preferências não sensíveis)

**Testing**: Vitest 5 (unitário e integração), OpenSSH real em contêiner Docker
(`tests/fixtures/sshd`), Playwright 1.64 com Electron (E2E)

**Target Platform**: Windows 10 e 11 x64 (vidro: Windows 11 22H2+; demais caem no fallback)

**Project Type**: desktop-app (Electron, projeto único)

**Performance Goals**: conexão LAN < 3 s; busca em 1.000 hosts < 200 ms/tecla; troca de aba
< 100 ms com 20 sessões; SFTP ≥ 80% da velocidade do `sftp` CLI

**Constraints**: offline (sem rede além do host-alvo), nenhum segredo fora do cofre, nenhum
log em arquivo, interface sem acesso a Node, contraste ≥ 4,5:1 sobre vidro, pt-BR

**Scale/Scope**: usuário único; até ~1.000 hosts; 20+ abas simultâneas; ~15 telas/diálogos

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Verificação | Status |
|---|---|---|
| **I. Clean Room** | Nenhum código/asset de referência; ícones SVG e visual próprios; sem bibliotecas de componentes; todas as dependências MIT/ISC/Apache-2.0 (research, cabeçalho). `docs/reference` só descreve comportamento. | ✅ |
| **II. Segurança** | Argon2id + XChaCha20-Poly1305 via libsodium, sem cripto própria (R9); segredos só no principal, nunca devolvidos à interface (contrato); sem logs (R12); host key obrigatória com bloqueio em divergência (R5, data-model KnownHostKey); `sandbox`/`contextIsolation`, zod em toda entrada, CSP, verificação de sender (R10); área de transferência só em ação explícita (`copyPublicKey`). Revisão de segurança explícita: ver seção abaixo. | ✅ |
| **III. Local-First** | Nenhum serviço online, sem telemetria, `crashReporter` desligado; formato do cofre com `formatVersion` e regra de migração (data-model). | ✅ |
| **IV. Testes Primeiro** | Unitários antes da implementação para cofre, cripto, hosts/grupos, busca, regras e chaves; integração SSH/SFTP/forwarding contra OpenSSH em contêiner; varredura de segredos (SC-004). | ✅ |
| **V. Simplicidade** | Escopo = spec; itens "fora" não tocados. `ssh2` e `node-pty` em vez de protocolo próprio. Uma conexão por aba (sem pool). Dependências extras justificadas em Complexity Tracking. | ✅ |
| Arquitetura | Principal / interface sem privilégios / ponte tipada; TypeScript strict; docs pt-BR, código em inglês. | ✅ |

**Re-check pós-design (Phase 1)**: data-model, contrato IPC e quickstart mantêm todas as
verificações acima. Nenhuma violação; Complexity Tracking registra apenas justificativas de
dependências.

### Revisão de segurança (Princípio II)

Superfícies tocadas: cofre, criptografia, autenticação SSH, ponte IPC. Riscos e mitigação:

| Risco | Mitigação |
|---|---|
| Força bruta offline do `vault.svlt` | Argon2id 256 MiB / 3 passes; senha ≥ 8 caracteres; parâmetros no cabeçalho permitem endurecer depois |
| Troca de parâmetros KDF ou rollback do cabeçalho | Cabeçalho é *additional data* AEAD; adulteração = falha de desbloqueio |
| Reuso de nonce | Nonce aleatório de 192 bits (XChaCha) novo a cada gravação |
| Corrupção por queda de energia | Escrita em temporário + `fsync` + `rename`; `vault.bak` |
| Segredos residuais em memória após bloqueio | `memzero` em DEK/KEK; estado decifrado descartado; strings JS não são zeráveis — risco aceito e documentado (mesmo patamar de apps Electron equivalentes) |
| Interface comprometida (XSS via nome de arquivo remoto, banner SSH) | React escapa texto; CSP sem `unsafe-inline` em scripts; renderer sandboxed; API não expõe leitura de segredos nem caminho local arbitrário |
| Mensagens de erro vazando segredos | `AppError` com mensagens pt-BR fixas por código; erros brutos do ssh2 nunca repassados inteiros |
| MITM na primeira conexão | Impressão digital SHA-256 exibida; divergência bloqueia com alerta (SC-005) |
| Port forwarding exposto na rede | Padrão `127.0.0.1`; outra interface exige opção explícita com aviso |
| Binário de shell falso no PATH | `powershell.exe`/`cmd.exe` por caminho absoluto em `%SystemRoot%\System32` |

## Project Structure

### Documentation (this feature)

```text
specs/001-mvp-ssh-client/
├── plan.md              # este arquivo
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── ipc-bridge.md    # Phase 1 — API window.shellvault
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
src/
├── main/                    # processo principal (Node, privilegiado)
│   ├── index.ts             # ciclo de vida, janela, segurança da webContents
│   ├── appearance.ts        # acrílico/fallback, nativeTheme
│   ├── settings.ts          # settings.json
│   ├── vault/               # formato, crypto (libsodium), store, auto-lock, migrações
│   ├── domain/              # hosts, groups, credentials, knownHosts, forwardRules (lógica pura)
│   ├── ssh/                 # connection (ssh2 + hostVerifier + auth), terminal session
│   ├── local/               # pty (node-pty)
│   ├── sftp/                # browser ops, transfers (fastPut/fastGet, parciais, conflitos)
│   ├── forwarding/          # net.Server por regra, conexão por host
│   └── ipc/                 # handlers por módulo, validação zod, Result/AppError
├── preload/
│   └── index.ts             # contextBridge → window.shellvault
├── shared/                  # tipos e schemas compartilhados (sem dependências de Node/DOM)
│   ├── ipc/                 # contrato tipado, ErrorCode
│   └── model/               # tipos de entidades, views
└── renderer/                # interface (sem privilégios)
    ├── index.html
    ├── main.tsx
    ├── app/                 # layout, roteamento de telas, confirmação ao fechar
    ├── stores/              # Zustand: vault, hosts, sessions, transfers, forwards
    ├── features/
    │   ├── vault/           # criar, desbloquear, trocar senha, apagar
    │   ├── hosts/           # barra lateral, grupos, busca, formulários, credenciais
    │   ├── terminal/        # abas, xterm, diálogos de host key e prompts
    │   ├── sftp/            # navegador, fila de transferências, conflitos
    │   ├── forwarding/      # regras e status
    │   └── settings/
    ├── ui/                  # componentes próprios (botão, campo, diálogo, menu), ícones SVG
    ├── theme/               # tokens (vidro/sólido, claro/escuro, alto contraste)
    └── i18n/pt-BR.ts

tests/
├── unit/                    # espelha src/main/{vault,domain}, src/shared, renderer/theme e busca
├── integration/             # ssh, sftp, forwarding, local pty contra fixtures
├── e2e/                     # Playwright + Electron
└── fixtures/
    ├── sshd/                # Dockerfile + compose do OpenSSH de teste, servidor de eco
    └── keys/                # chaves de teste (geradas no setup, nunca reais)
```

**Structure Decision**: projeto único Electron no layout padrão do electron-vite
(`src/main`, `src/preload`, `src/renderer`) mais `src/shared` para o contrato da ponte, que
é importado pelos dois lados e garante tipagem ponta a ponta. A lógica de domínio
(`src/main/domain`, `src/main/vault`) não depende de Electron, para ser testada com Vitest
puro.

## Complexity Tracking

Sem violações da constituição. Justificativas de dependências exigidas pelo Princípio V:

| Dependência / decisão | Por que é necessária | Alternativa mais simples rejeitada porque |
|---|---|---|
| Zustand | Estado de abas/sessões/transferências compartilhado entre barra lateral, abas e diálogos com 20+ sessões | Context do React re-renderiza a árvore inteira a cada byte de progresso |
| zod | Validação de toda entrada da ponte (Princípio II) e de `settings.json` | Validação manual é verbosa e fácil de esquecer em um campo |
| libsodium-wrappers-sumo | Argon2id + XChaCha20-Poly1305 auditados | `node:crypto` não tem Argon2id nem XChaCha |
| Addons xterm (webgl, unicode11, fit) | SC-007 e FR-015 (Unicode, redimensionamento) | Renderer DOM fica lento com 20 abas e saída intensa |
| Docker para testes de integração | Princípio IV exige servidor SSH real | `ssh2.Server` em processo não é servidor real |
| Conexão compartilhada por host para forwarding | Várias regras do mesmo host sem N conexões | Uma conexão por regra multiplica autenticações e prompts de host key |
