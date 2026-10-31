# Implementation Plan: Migração do ShellVault para Go + Wails (Fase 1)

**Branch**: `002-wails-migration` | **Date**: 2026-10-23 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/002-wails-migration/spec.md`

## Summary

Reescrever o processo principal do ShellVault em Go sobre Wails v2.17, reaproveitando a
interface React/xterm.js da 001, com paridade total (cofre, hosts, grupos, credenciais, SSH,
shell local, SFTP, encaminhamento local, confirmação ao fechar). O cofre passa de arquivo
único para SQLite com conteúdo cifrado por linha, mantendo Argon2id + XChaCha20-Poly1305 e
migrando o `vault.svlt` existente. Os quatro bugs da versão Electron viram requisitos com teste
de regressão (instância/processos, assinatura única de saída e resize após fit, layout com
ResizeObserver, menus ancorados com detecção de colisão). Entram keepalive, algoritmos legados
por host, agente SSH, atalhos e paleta de comandos, busca no terminal, personalização com
perfis por host e importação de esquemas, e a identidade visual gerada pelo `brand-kit`.
Decisões em [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.26.x (`CGO_ENABLED=0`); TypeScript 6 `strict` no frontend; Node 24
LTS só para build

**Primary Dependencies**: Wails v2.17 (MIT); `golang.org/x/crypto` (ssh, ssh/agent,
ssh/knownhosts, argon2, chacha20poly1305 — BSD-3); `github.com/pkg/sftp` (BSD-2);
`modernc.org/sqlite` (BSD-3); `github.com/aymanbagabas/go-pty` (MIT);
`github.com/Microsoft/go-winio` (MIT); `golang.org/x/image/font/sfnt` (BSD-3);
`go.uber.org/goleak` (MIT, só testes). Frontend: React 19, Zustand 5, `@xterm/xterm` 6 + addons
fit/webgl/unicode11/search/web-links, `@floating-ui/react-dom` (MIT). Fontes JetBrains Mono e
Cascadia Mono/Code (OFL-1.1).

**Storage**: `%APPDATA%\ShellVault\shellvault.db` (SQLite WAL; entidades do cofre cifradas
por linha; preferências, perfis e esquemas em claro); importação única de `vault.svlt` +
`settings.json` da 001

**Testing**: `go test` (unitário), `go test -tags integration` contra OpenSSH em Docker
(`tests/fixtures/sshd` + novos `sshd-legacy` e `idle-killer`), `goleak`; Vitest no frontend;
Playwright via CDP no WebView2 do build `e2e`

**Target Platform**: Windows 10/11 x64 (vidro: Windows 11; Windows 10 cai no sólido); código
sem bloqueios para Linux/macOS, que ficam sem build nesta fase

**Project Type**: desktop-app (Wails: backend Go + frontend web embutido)

**Performance Goals**: prompt e eco sem atraso perceptível; troca de aba < 100 ms com 20
sessões e uma em saída contínua; personalização aplicada em < 1 s; busca em 1.000 hosts
< 200 ms; SFTP ≥ 001

**Constraints**: offline; nenhum segredo legível no disco; nenhum log/crash dump em arquivo;
instalador < 30 MB; memória ociosa com 1 aba < 150 MB (risco R19); contraste ≥ 4,5:1; pt-BR;
nenhuma goroutine vazada por sessão

**Scale/Scope**: usuário único; ~1.000 hosts; 20+ abas; ~20 telas/diálogos; 8 serviços na ponte

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Verificação | Status |
|---|---|---|
| **I. Clean Room** | Nenhum código de referência; o frontend da 001 é código próprio; logo e tokens novos do `brand-kit` (o skill em si é ferramenta de dev, fora do repositório e do binário). Todas as dependências MIT/BSD/OFL (R1–R16); presets de cor usam só valores publicados sob MIT, com origem registrada. | ✅ |
| **II. Segurança** | Argon2id + XChaCha20-Poly1305 da `x/crypto`, sem cripto própria (R4); entidades do cofre cifradas por linha com AD amarrado a tabela+id (R3); nenhum método da ponte devolve segredo e o conjunto de métodos é verificado por teste (contrato); host key obrigatória com bloqueio (R6); WebView2 sem acesso ao sistema, CSP sem script inline; porta de depuração só no build `e2e`, com teste que garante ausência no release (R18); algoritmos legados desligados por padrão e sinalizados. Revisão de segurança abaixo. | ✅ |
| **III. Local-First** | Sem serviço online, sem telemetria, logs só em stdout, WebView2 sem crash dumps (R17); esquema SQLite versionado (`user_version`) e migração documentada e testada da 001 (R5). | ✅ |
| **IV. Testes Primeiro** | Unitários antes do código para cofre, migração, domínio, algoritmos, agregador/UTF-8, importadores; integração SSH/SFTP/forwarding/legados/keepalive/agente contra contêineres; cada um dos 4 bugs ganha teste de regressão; varredura de segredos. | ✅ |
| **V. Simplicidade** | Escopo = spec (Fase 2/3 fora). Bibliotecas consolidadas em vez de protocolo próprio. Reuso do frontend da 001. Dependências novas justificadas em Complexity Tracking. | ✅ |
| Arquitetura | Processo Go (SO, SSH, cofre) / interface WebView2 sem privilégios / ponte Wails com métodos validados. Tipagem estática (Go + TS strict). Docs pt-BR, código em inglês. | ✅ com emenda PATCH (R20): exemplos de bibliotecas e linguagem da constituição citam Node/TypeScript; atualizar para Go antes da implementação |

**Re-check pós-design (Phase 1)**: data-model (cifra por linha, migração), contrato (serviços
fechados, `TestService` só no build `e2e`) e quickstart mantêm todas as verificações. Nenhuma
violação de princípio; a emenda R20 é de redação.

### Revisão de segurança (Princípio II)

Superfícies tocadas: cofre (novo contêiner e migração), criptografia (nova implementação),
autenticação SSH (agente, algoritmos legados), ponte (novo mecanismo).

| Risco | Mitigação |
|---|---|
| Implementação nova da cripto divergir da 001 | Mesmas primitivas da `x/crypto`; teste abre cofre real gerado pela 001 e compara campo a campo |
| Linhas cifradas trocadas entre si ou entre tabelas | AD = `"<tabela>:<id>"`; troca → falha de autenticação |
| Metadados em claro revelarem informação | Colunas em claro só com ids, ordem e datas; nomes, endereços e usuários cifrados |
| Páginas apagadas do SQLite com dados antigos | `secure_delete=ON`; WAL com checkpoint ao bloquear |
| Migração deixar o cofre antigo e o novo inconsistentes | Grava em `.tmp`, verifica, renomeia; original só é renomeado (nunca apagado) após verificação |
| Algoritmos fracos (SHA-1, CBC) expostos | Desligados por padrão, só por host, modernos preferidos, aviso visual permanente |
| Agente SSH malicioso ou ausente | Só o pipe do OpenSSH do Windows; erro `AGENT_UNAVAILABLE`; o app não encaminha o agente para o servidor (sem *agent forwarding* nesta fase) |
| Ponte expondo métodos demais | Só 8 structs ligadas; teste de reflexão compara métodos com o contrato |
| Porta de depuração do WebView2 em produção | Habilitada apenas pela tag `e2e`; teste no binário de release |
| URLs no terminal abrindo conteúdo malicioso | Só `http`/`https`, só com Ctrl+clique, abre no navegador externo; o app não faz requisição |
| Colagem de comandos perigosos | Confirmação para várias linhas com caracteres de controle exibidos escapados |
| Segredos residuais em memória | DEK/KEK em `[]byte` zerados no bloqueio; strings não zeráveis — risco aceito como na 001 |
| Crash dumps do WebView2 com conteúdo de memória | `--disable-breakpad`/`--disable-crash-reporter`; teste varre o diretório de dados |
| Autofill do WebView2 persistir formulários e chaves em claro | Desabilitar autofill geral e salvamento de senhas via `ICoreWebView2Settings4`; remover `Web Data` e seus sidecars legados antes de criar o WebView |
| Binário de shell falso no PATH | Caminho absoluto em `%SystemRoot%\System32` (como na 001) |

## Project Structure

### Documentation (this feature)

```text
specs/002-wails-migration/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── bridge.md
├── checklists/
│   └── requirements.md
└── tasks.md              # /speckit-tasks
```

### Source Code (repository root)

```text
wails.json
go.mod / go.sum
main.go                     # wails.Run: opções de janela, single instance, Bind dos serviços
internal/
├── bridge/                 # structs ligadas ao Wails (VaultService…AppService), Result, ErrorCode, validação
│   └── testsvc/            # TestService (//go:build e2e)
├── vault/                  # crypto (argon2/xchacha), store SQLite, schema e migrações, lock/autolock
│   └── migrate001/         # leitura do vault.svlt v1 + settings.json da 001
├── domain/                 # hosts, groups, credentials, knownhosts, forwardrules, profiles, schemes (regras puras)
├── session/                # Manager, Session, agregador de saída, decodificador UTF-8
├── sshx/                   # dial, auth, algoritmos legados, keepalive, host key, agente
├── localpty/               # interface PTY + shells (powershell/cmd)
├── sftpx/                  # navegação, transferências, parciais, conflitos
├── forwarding/             # regras ativas, listeners, uma conexão por host
├── schemes/                # importadores Windows Terminal / iTerm2, exportador, presets
├── fonts/                  # varredura de fontes monoespaçadas
└── appwin/                 # aparência do Windows, confirmação ao fechar, preferências
frontend/                   # React (migrado de src/renderer da 001)
├── src/
│   ├── app/ features/ stores/ ui/ i18n/
│   ├── theme/              # tokens.css gerado pelo brand-kit + aplicação de modo
│   └── bridge/             # adaptador sobre wailsjs/go/* (gerado) mantendo a forma Result<T>
├── wailsjs/                # bindings gerados pelo Wails (não editar)
└── tests/                  # vitest + e2e (Playwright via CDP)
build/                      # appicon.png, windows/icon.ico, installer (do kit da marca)
brand/                      # brand.json, kit do logo, brand book (saídas do brand-kit)
tests/fixtures/
├── sshd/                   # da 001
├── sshd-legacy/            # novo: só algoritmos legados
├── idle-killer/            # novo: derruba conexões ociosas
└── vault-001/              # cofre e settings gerados pela versão Electron
scripts/                    # sshd.mjs (com os novos contêineres), bench-sftp
src/, electron-builder.yml… # base Electron: permanece até a tarefa final de remoção (pós-paridade)
```

**Structure Decision**: layout padrão do Wails (raiz com `main.go`, `wails.json`, `frontend/`,
`build/`) e pacotes Go em `internal/`. O frontend é movido de `src/renderer` e `src/shared` para
`frontend/` no início, preservando o histórico (`git mv`); a camada `window.shellvault` é
substituída por um adaptador sobre os bindings gerados, então features e stores mudam pouco. A
base Electron (`src/main`, `src/preload`, configs) continua no repositório como referência
executável até a validação de paridade e é removida numa tarefa final.

## Fases de implementação (ordem para `/speckit-tasks`)

1. **Fundação**: emenda da constituição; scaffold Wails; mover frontend; adaptador da ponte;
   CI (go vet/test, lint do frontend); contêineres de teste novos.
2. **Cofre e migração** (US1): crypto, store SQLite, migração 001 com fixture.
3. **Sessões e SSH** (US1): Manager, agregador, host key, auth, prompt único, fechar sem vazamento.
4. **Layout e menus** (US2): grid, ResizeObserver, popover ancorado, barra de título própria.
5. **NOC** (US3): keepalive, legados, agente, atalhos, paleta, busca, colagem, links.
6. **Paridade restante** (US6 + 001): grupos, credenciais, shell local, SFTP, forwarding,
   fechar, aparência.
7. **Marca** (US5): `brand-kit` estágios 1–5 → tokens, ícone, aplicação em todas as telas;
   ícones Lucide, vidro e estados nos componentes, movimento, barra lateral recolhível em trilho,
   página de configurações, cor de destaque e barra de status (research R16a).
8. **Personalização** (US4): perfis, esquemas, importadores, preview, override por host.
9. **Polimento**: varredura de segredos, tamanho/memória, revisão de segurança, licenças,
   validação do quickstart, instalador, remoção da base Electron.

## Complexity Tracking

| Item | Por que é necessário | Alternativa mais simples rejeitada porque |
|---|---|---|
| SQLite com cifra por linha (em vez de arquivo único) | Escolha do usuário; gravações incrementais e transacionais | Hosts em claro violariam FR-002; SQLCipher exige CGO |
| `go-winio` | Único jeito de falar com o agente do OpenSSH no Windows (named pipe) | Sem ele, FR-124 não existe no Windows |
| `@floating-ui/react-dom` | Posicionamento com colisão foi a origem do bug 4 | Cálculo manual já falhou na 001 |
| `x/image/font/sfnt` | Listar fontes monoespaçadas instaladas (FR-152) | API de fontes locais do navegador exige permissão não exposta pelo Wails v2 |
| Build `e2e` com `TestService` e CDP | E2E de layout e vazamento exigem controlar a janela e ler contadores | Testes só manuais não cobrem 20 ciclos × 3 escalas de forma repetível |
| Contêineres `sshd-legacy` e `idle-killer` | SC-110 e SC-111 precisam de servidor real com algoritmos antigos e de queda por ociosidade | Mock do protocolo não prova interoperabilidade |
| Patch local do `go-webview2` v1.0.22 (`third_party/go-webview2`, `replace` no `go.mod`) | O loader zera `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` e o Wails v2.17 não expõe argumentos do navegador; sem eles não há `--disable-breakpad` (FR-030) nem porta CDP para os E2E. Patch de 1 variável, documentado em `PATCH.md` | Variável de ambiente e registro são ignorados pelo loader (verificado na T017); Wails v3 é beta |
| Base Electron mantida até a paridade | Referência executável para comparar comportamento | Remover antes perde a comparação lado a lado |

### Riscos

| Risco | Impacto | Plano |
|---|---|---|
| Memória do WebView2 > 150 MB (SC-109) | Critério não atendido | Medir cedo (fase 1); mitigação em R19; se impossível, reportar com números e pedir decisão |
| Snap layouts ao passar o mouse indisponíveis no v2 | Diferença de UX vs. janela nativa | Registrado na spec; reavaliar quando o Wails v3 for estável |
| Equipamentos reais indisponíveis para SC-102 | Critério sem prova em hardware | Regressão automática do prompt único com servidores em contêiner + roteiro manual com o usuário |
| Go não instalado na máquina de desenvolvimento | Bloqueia a fase 1 | Primeira tarefa: instalar Go, Wails CLI e NSIS (com aprovação do usuário) |
